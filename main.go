package main

import (
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"sync"
	"time"

	client_native "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/models/v2"
	"go.uber.org/zap"
)

func main() {
	// Create logger
	logger, err := getLogger()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	logger.Info("Starting haproxy process")
	cmd, err := startHaproxyProcess()
	if err != nil {
		logger.Fatal("failed to start HaproxyProcess", zap.Error(err))
	}

	logger.Info("Creating haproxy client")
	client, err := getHaproxyClient()
	if err != nil {
		logger.Fatal("Failed to create haproxy client", zap.Error(err))
	}

	logger.Info("Starting watchdog")
	if err := watchdog(logger, client, cmd); err != nil {
		logger.Fatal("Watchdog failed", zap.Error(err))
	}
}

func watchdog(logger *zap.Logger, client *client_native.HAProxyClient, cmd *exec.Cmd) error {
	conf := client.GetConfiguration()

	checkURL := mustParseURL("https://httpbin.org/anything")

	if err := getNewProxies(logger, conf); err != nil {
		logger.Error("couldn't get new proxies", zap.Error(err))
	}
	if err := testCurrentProxies(logger, conf, cmd, checkURL); err != nil {
		logger.Error("couldn't get check current proxies", zap.Error(err))
	}

	cpTicker := time.Tick(time.Second * 30)
	npTicker := time.Tick(time.Minute * 1)

	for {
		select {
		case <-npTicker:
			err := getNewProxies(logger, conf)
			if err != nil {
				logger.Error("couldn't get new proxies", zap.Error(err))
			}
		case <-cpTicker:
			logger.Debug("We are checking current proxies")
			err := testCurrentProxies(logger, conf, cmd, checkURL)
			if err != nil {
				logger.Error("couldn't test current proxies", zap.Error(err))
			}
		}
	}

	return nil
}

func testCurrentProxies(
	logger *zap.Logger,
	conf client_native.IConfigurationClient,
	cmd *exec.Cmd, target *url.URL) error {
	version, err := conf.GetVersion("")
	if err != nil {
		return err
	}
	// We will start with a transaction to make sure that it's good
	trans, err := conf.StartTransaction(version)
	if err != nil {
		return err
	}
	defer func() {
		logger.Debug("deleting transaction", zap.String("tid", trans.ID))
		conf.DeleteTransaction(trans.ID)
	}()

	logger.Debug("starting transaction", zap.String("tid", trans.ID))

	// Here we must get current proxies
	_, servers, err := conf.GetServers("sk-backend", trans.ID)
	if err != nil {
		return err
	}

	var srvresMtx sync.Mutex
	srvres := make(map[*models.Server]*ProxyTestResult, 0)

	var wg sync.WaitGroup
	for _, srv := range servers {
		wg.Add(1)
		go func(srv *models.Server) {
			defer wg.Done()

			lgs := logger.With(zap.String("name", srv.Name))
			lgs.Debug("examining server",
				zap.String("address", srv.Address), zap.Int64("port", *srv.Port))

			ans, err := testProxy(lgs, srv, target)
			if err != nil {
				lgs.Error("something went wrong in test proxy", zap.Error(err))
				// If this errors something is very wrong.
				// return err
				return
			}
			lgs.Debug("got ans", zap.Reflect("ans", ans))

			srvresMtx.Lock()
			srvres[srv] = ans
			srvresMtx.Unlock()

		}(srv)
	}
	wg.Wait()

	// Now we can decide which ones we want to enable or disable
	serverAction := make(map[*models.Server]string, 0)

	// We map over these to remove any who failed
	for srv, pt := range srvres {
		// The once that fail we delete for now
		if pt.Failed {
			serverAction[srv] = "delete"
		}

		if pt.TotalDur > 2*time.Second || pt.StatusCode != http.StatusOK {
			if srv.Maintenance == "enabled" {
				// Remove the proxy second time around
				serverAction[srv] = "delete"
			} else {
				srv.Maintenance = "enabled"
				serverAction[srv] = "edit"
			}
		}

		// Now we enabled the ones which are left
		if srv.Maintenance == "enabled" {
			srv.Maintenance = "disabled"
			serverAction[srv] = "edit"
		}
	}

	// Here we edit the servers.
	for srv, action := range serverAction {
		if action == "edit" {
			if err := conf.EditServer(srv.Name, "sk-backend", srv, trans.ID, 0); err != nil {
				return err
			}
		} else if action == "delete" {
			if err := conf.DeleteServer(srv.Name, "sk-backend", trans.ID, 0); err != nil {
				return err
			}
		} else {
			logger.Error("We don't know this value", zap.String("action", action))
		}
	}

	if len(serverAction) < 0 {
		return nil
	}

	if _, err := conf.CommitTransaction(trans.ID); err != nil {
		return err
	}
	logger.Debug("commited transaction", zap.String("tid", trans.ID))

	if err := reloadHaproxyProcess(cmd); err != nil {
		return err
	}
	logger.Debug("Reloaded haproxy process")

	return nil
}

func getNewProxies(
	logger *zap.Logger,
	conf client_native.IConfigurationClient) error {
	logger.Info("We are checking for new proxies")

	version, err := conf.GetVersion("")
	if err != nil {
		return err
	}
	// We will start with a transaction to make sure that it's good
	trans, err := conf.StartTransaction(version)
	if err != nil {
		return err
	}
	defer func() {
		logger.Debug("deleting transaction", zap.String("tid", trans.ID))
		conf.DeleteTransaction(trans.ID)
	}()

	logger.Debug("starting transaction", zap.String("tid", trans.ID))

	xproxies, err := getFireXProxies()
	if err != nil {
		return err
	}

	_, servers, err := conf.GetServers("sk-backend", trans.ID)
	if err != nil {
		return err
	}

	newServers := models.Servers{}

OUTER:
	for _, xprox := range xproxies {
		if xprox.Protocol != "SOCKS5" {
			continue
		}
		for _, srv := range servers {
			if srv.Address == xprox.Server && *srv.Port == int64(xprox.Port) {
				continue OUTER
			}
		}

		// add it to the servers but in a down state. The test current will update them and test
		srv := &models.Server{
			Address:     xprox.Server,
			Port:        pInt64(int64(xprox.Port)),
			Name:        genSecureRandomName(),
			Maintenance: "enabled",
		}
		newServers = append(newServers, srv)
	}

	if len(newServers) == 0 {
		return nil
	}

	// If there are new servers we add them.
	for _, srv := range newServers {
		logger.Info("adding server to haproxy",
			zap.String("name", srv.Name),
			zap.String("address", srv.Address),
			zap.Int64("port", *srv.Port))

		if err := conf.CreateServer("sk-backend", srv, trans.ID, 0); err != nil {
			return err
		}
	}

	if _, err := conf.CommitTransaction(trans.ID); err != nil {
		return err
	}
	logger.Debug("commited transaction", zap.String("tid", trans.ID))

	return nil
}

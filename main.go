package main

import (
	"log"
	"net/url"
	"os/exec"
	"time"

	client_native "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/models/v2"
)

func main() {
	cmd, err := startHaproxyProcess()
	if err != nil {
		log.Fatal(err)
	}

	client, err := getHaproxyClient()
	if err != nil {
		log.Fatal(err)
	}

	if err := watchdog(client, cmd); err != nil {
		log.Fatal(err)
	}

}

func watchdog(client *client_native.HAProxyClient, cmd *exec.Cmd) error {
	conf := client.GetConfiguration()

	checkUrl := mustParseUrl("https://httpbin.org/anything")

	getNewProxies(conf)

	cpTicker := time.Tick(time.Second * 10)
	npTicker := time.Tick(time.Minute * 1)

	for {
		select {
		case <-npTicker:
			log.Printf("We will check for new proxies\n")
			err := getNewProxies(conf)
			if err != nil {
				log.Printf("We had an error: %s\n", err.Error())
			}
		case <-cpTicker:
			log.Printf("We will check current proxies\n")
			err := testCurrentProxies(conf, cmd, checkUrl)
			if err != nil {
				log.Printf("We had an error: %s\n", err.Error())
			}
		}
	}

	return nil
}

func testCurrentProxies(conf client_native.IConfigurationClient,
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
	defer conf.DeleteTransaction(trans.ID)

	// Here we must get current proxies
	_, servers, err := conf.GetServers("sk-backend", trans.ID)
	if err != nil {
		return err
	}

	for _, server := range servers {
		log.Printf("Examining server: %s\n", server.Address)
	}

	return nil
}

func getNewProxies(conf client_native.IConfigurationClient) error {
	version, err := conf.GetVersion("")
	if err != nil {
		return err
	}
	// We will start with a transaction to make sure that it's good
	trans, err := conf.StartTransaction(version)
	if err != nil {
		return err
	}
	defer conf.DeleteTransaction(trans.ID)

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
			Name:        genRandomName(),
			Maintenance: "enabled",
		}
		newServers = append(newServers, srv)
	}

	if len(newServers) == 0 {
		return nil
	}

	// If there are new servers we add them.
	for _, srv := range newServers {
		if err := conf.CreateServer("sk-backend", srv, trans.ID, 0); err != nil {
			return err
		}
	}

	if _, err := conf.CommitTransaction(trans.ID); err != nil {
		return err
	}

	return nil
}

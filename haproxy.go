package main

import (
	"errors"
	"fmt"
	"log"
	"net/url"

	client_native "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/client-native/v2/configuration"
	"github.com/haproxytech/client-native/v2/runtime"
	"github.com/haproxytech/models/v2"
)

func getHaproxyClient() (*client_native.HAProxyClient, error) {
	confClient := &configuration.Client{}
	confParams := configuration.ClientParams{
		ConfigurationFile:      "haproxy-stuff/conf/haproxy.cfg",
		Haproxy:                "haproxy-stuff/bin/haproxy",
		UseValidation:          true,
		PersistentTransactions: true,
		TransactionDir:         "haproxy-stuff/txns",
		MasterWorker:           true,
	}

	if err := confClient.Init(confParams); err != nil {
		return nil, err
	}

	runtimeClient := &runtime.Client{}
	_, globalConf, err := confClient.GetGlobalConfiguration("")
	if err != nil {
		return nil, err
	}
	// log.Printf("We got configuration with gnum: %d\n", gnum)
	if len(globalConf.RuntimeAPIs) != 0 {
		socketList := make([]string, 0)
		for _, r := range globalConf.RuntimeAPIs {
			socketList = append(socketList, *r.Address)
		}
		if err := runtimeClient.Init(socketList, "", 0); err != nil {
			// log.Fatalf("Error setting up runtime client: %s\n", err.Error())
			return nil, err
		}
	} else {
		log.Println("No runtime API configured, not using it")
		runtimeClient = nil
	}

	client := &client_native.HAProxyClient{}
	client.Init(confClient, runtimeClient)

	return client, nil
}

func setupBaseConfig(conf client_native.IConfigurationClient) error {
	version, err := conf.GetVersion("")
	if err != nil {
		return err
	}
	log.Printf("We got version: %d\n", version)

	trans, err := conf.StartTransaction(version)
	if err != nil {
		return err
	}
	log.Printf("We are starting transaction: %s\n", trans.ID)

	_, frontend, err := conf.GetFrontend("powerplay", trans.ID)
	if err != nil {
		cerr := &configuration.ConfError{}
		if !errors.As(err, &cerr) {
			return err
		}
		if cerr.Code() != configuration.ErrObjectDoesNotExist {
			return err
		}
		log.Printf("The frontend doesn't exist, we are adding it\n")
		frontend = &models.Frontend{
			Name: "powerplay",
		}
		if err := conf.CreateFrontend(frontend, trans.ID, 0); err != nil {
			return err
		}
	}

	frontend.Mode = "tcp"
	frontend.Tcplog = true

	if err := conf.EditFrontend("powerplay", frontend, trans.ID, 0); err != nil {
		return err
	}

	mtrans, err := conf.CommitTransaction(trans.ID)
	if err != nil {
		return err
	}
	log.Printf("Commited txn %s, status: %s\n", mtrans.ID, mtrans.Status)

	return nil
}

func haGetCurrentProxies(conf client_native.IConfigurationClient, name string) ([]*url.URL, error) {
	_, servs, err := conf.GetServers(name, "")
	if err != nil {
		return nil, err
	}

	uls := make([]*url.URL, 0)
	for _, srv := range servs {
		log.Printf("We have a server: %s\n", srv.Address)
		ul, err := url.Parse(fmt.Sprintf("socks5://%s:%d", srv.Address, srv.Port))
		if err != nil {
			return nil, err
		}
		uls = append(uls, ul)
	}
	return uls, nil
}

package main

import (
	"log"
	"time"

	client_native "github.com/haproxytech/client-native/v2"
)

func main() {
	client, err := getHaproxyClient()
	if err != nil {
		log.Fatal(err)
	}

	if err := watchdog(client); err != nil {
		log.Fatal(err)
	}

	// conf := client.GetConfiguration()

	// if err := setupBaseConfig(conf); err != nil {
	// 	log.Fatal(err)
	// }
}

func watchdog(client *client_native.HAProxyClient) error {
	conf := client.GetConfiguration()
	conf = conf

	run := client.GetRuntime()

	for {
		log.Println("Entered watchdog loop")
		curp, err := haGetCurrentProxies(conf, "sk-backend")
		if err != nil {
			return err
		}
		log.Println(curp)

		info, err := run.GetInfo()
		if err != nil {
			return err
		}
		for _, inf := range info {
			log.Printf("Here we are!\n")
			log.Printf("The pid is: %d\n", *inf.Info.Pid)
		}

		time.Sleep(10 * time.Second)
	}
	return nil
}

package main

import (
	"log"

	"github.com/kelseyhightower/envconfig"

	"github.com/Southclaws/machinehead/server"
)

func main() {
	config := server.Config{}
	err := envconfig.Process("MACHINEHEAD", &config)
	if err != nil {
		log.Fatal(err)
	}

	app, err := server.Initialise(config)
	if err != nil {
		log.Fatal(err)
	}

	app.Start()
}

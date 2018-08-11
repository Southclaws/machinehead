package main

import (
	"log"

	"github.com/kelseyhightower/envconfig"

	"github.com/Southclaws/Machinehead/server"
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

	log.Fatal(app.Start())
}

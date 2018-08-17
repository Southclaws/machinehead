package main

import (
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"
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

	os.Exit(app.Start())
}

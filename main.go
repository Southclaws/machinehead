package main

import (
	"fmt"
	"os"

	"github.com/Southclaws/machinehead/server"
)

func main() {
	config, err := server.LoadConfig()
	if err != nil {
		fmt.Println("failed to load config:", err)
		os.Exit(1)
	}

	app, err := server.Initialise(config)
	if err != nil {
		fmt.Println("failed to initialise:", err)
		os.Exit(2)
	}

	os.Exit(app.Start())
}

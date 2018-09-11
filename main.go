package main

import (
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"go.uber.org/zap"

	"github.com/Southclaws/machinehead/server"
)

func main() {
	var logger *zap.Logger

	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:        "run",
			Aliases:     []string{"r"},
			Description: "Begin watching targets for changes, it is recommended that this process is daemonised.",
			Action: func(c *cli.Context) (err error) {
				logger = setupLogger(c.GlobalBool("verbose"))

				config, err := server.LoadConfig()
				if err != nil {
					return errors.Wrap(err, "failed to load config")
				}

				s, err := server.Initialise(config, logger)
				if err != nil {
					if err == server.ErrExistingDaemon {
						logger.Info("Did you mean to use `run` or one of the daemon control commands?")
					}
					return errors.Wrap(err, "failed to initialise")
				}

				return s.Run()
			},
		},
		{
			Name:        "trigger",
			Aliases:     []string{"t"},
			Description: "Trigger all or some targets to run their commands.",
			Action: func(c *cli.Context) (err error) {
				// client.Dispatch()
				return
			},
		},
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "verbose",
			Usage:  "Enable more verbose messages",
			EnvVar: "VERBOSE",
		},
	}

	app.Before = func(c *cli.Context) (err error) {
		return
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Fatal("Exited with error", zap.String("error", err.Error()))
	}
}

func setupLogger(verbose bool) (logger *zap.Logger) {
	config := zap.NewDevelopmentConfig()
	if verbose {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	config.DisableStacktrace = true
	config.DisableCaller = true
	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return
}

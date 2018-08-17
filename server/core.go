package server

import (
	"context"

	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/Southclaws/machinehead/gitwatch"
)

// App stores application state
type App struct {
	Config  Config
	Watcher *gitwatch.Session
	Vault   *api.Client

	ctx context.Context
	cf  context.CancelFunc
}

// Initialise creates a new instance and prepares it for starting
func Initialise(config Config) (app *App, err error) {
	ctx, cf := context.WithCancel(context.Background())

	app = &App{
		Config: config,
		ctx:    ctx,
		cf:     cf,
	}

	app.Vault, err = api.NewClient(&api.Config{
		Address: config.VaultAddress,
	})
	if err != nil {
		err = errors.Wrap(err, "failed to create new vault client")
		return
	}
	app.Vault.SetToken(config.VaultToken)
	app.Vault.SetNamespace(config.VaultNamespace)

	app.Watcher, err = gitwatch.New(ctx, config.Targets, config.CheckInterval, config.CacheDirectory, true)
	if err != nil {
		cf()
		err = errors.Wrap(err, "failed to construct new git watcher")
		return
	}

	logger.Debug("starting machinehead with debug logging", zap.Any("config", config))

	return
}

// Start will start the application and block until graceful exit or fatal error
// returns an exit code to be passed back to the `main` caller for `os.Exit`.
func (app *App) Start() int {
	// first, bootstrap the repositories
	// pass errors to a channel
	errChan := make(chan error)
	go func() {
		errChan <- app.Watcher.Run()
	}()
	<-app.Watcher.InitialDone

	// initial `docker-compose up` of apps
	err := app.doInitialUp()
	if err != nil {
		logger.Fatal("daemon failed to initialise")
	}

	// start and block until error or graceful exit
	// always stop after, regardless of exit state
	defer app.Stop()
	err = app.start(errChan)
	if err != nil {
		logger.Error("daemon encountered an error",
			zap.Error(err))
		return 1
	}

	return 0
}

// Stop gracefully closes the application
func (app *App) Stop() {
	app.cf()

	for _, target := range app.Config.Targets {
		path, err := gitwatch.GetRepoPath(app.Config.CacheDirectory, target)
		if err != nil {
			continue
		}
		err = compose(path, map[string]string{}, "down")
		if err != nil {
			continue
		}

		logger.Info("shut down deployment",
			zap.String("target", target))
	}
}

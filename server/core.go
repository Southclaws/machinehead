package server

import (
	"context"
	"os"

	"github.com/Southclaws/gitwatch"
	"github.com/hashicorp/vault/api"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

// App stores application state
type App struct {
	Config      Config
	Targets     map[string]Target
	GlobalEnvs  map[string]string
	Watcher     *gitwatch.Session
	SelfWatcher *gitwatch.Session
	Vault       *api.Client
	Auth        transport.AuthMethod

	ctx context.Context
	cf  context.CancelFunc
}

// Initialise creates a new instance and prepares it for starting
func Initialise(config Config) (app *App, err error) {
	ctx, cf := context.WithCancel(context.Background())

	app = &App{
		Config: config,
		Targets: make(map[string]Target),
		ctx:    ctx,
		cf:     cf,
	}

	for _, t := range config.Targets {
		app.Targets[t.RepoURL] = t
	}

	_, err = os.Stat(".env")
	if err == nil {
		app.GlobalEnvs, err = godotenv.Read(".env")
		if err != nil {
			err = errors.Wrap(err, "failed to read global variables from .env")
			return
		}
	}

	if config.VaultAddress != "" {
		app.Vault, err = api.NewClient(&api.Config{
			Address: config.VaultAddress,
		})
		if err != nil {
			err = errors.Wrap(err, "failed to create new vault client")
			return
		}
		app.Vault.SetToken(config.VaultToken)
		if config.VaultNamespace != "" {
			app.Vault.SetNamespace(config.VaultNamespace)
		}

		_, err = app.Vault.Help("secret")
		if err != nil {
			err = errors.Wrap(err, "failed to perform request to vault server")
			return
		}
	}

	app.Auth, err = ssh.NewSSHAgentAuth("git")
	if err != nil {
		err = errors.Wrap(err, "failed to set up SSH authentication")
		return
	}

	err = app.setupGitWatcher()
	if err != nil {
		return
	}

	logger.Debug("starting machinehead with debug logging",
		zap.Any("config", config))

	return
}

// Start will start the application and block until graceful exit or fatal error
// returns an exit code to be passed back to the `main` caller for `os.Exit`.
func (app *App) Start() int {
	err := app.Run()
	if err != nil {
		logger.Error("application daemon encountered an error",
			zap.Error(err))
		return 1
	}
	return 0
}

// Run will run the application and block until graceful exit like `app.Start`
// but this function returns an explicit error. This is for use when Machinehead
// is being used as a library instead of a command line application.
func (app *App) Run() (err error) {
	// first, bootstrap the repositories
	// pass errors to a channel
	errChan := make(chan error)
	go func() {
		errChan <- app.Watcher.Run()
	}()
	select {
	case <-app.Watcher.InitialDone:
		break
	case err = <-errChan:
		return errors.Wrap(err, "git watcher encountered an error during initial clone")
	}

	// TODO: no more docker-compose
	// generic commands
	// add `InitialRun` check
	err = app.doInitialUp()
	if err != nil {
		return errors.Wrap(err, "daemon failed to initialise")
	}

	logger.Debug("done initial docker-compose up of targets")

	// start and block until error or graceful exit
	// always stop after, regardless of exit state
	defer app.Stop()
	err = app.start()
	if err != nil {
		return
	}

	return
}

// Stop gracefully closes the application
func (app *App) Stop() {
	// don't allow repeated graceful shutdowns
	if app.ctx.Err() != nil {
		return
	}

	logger.Debug("graceful shutdown initiated")

	app.cf()

	for _, target := range app.Config.Targets {
		path, err := gitwatch.GetRepoPath(app.Config.CacheDirectory, target.RepoURL)
		if err != nil {
			continue
		}
		err = target.Execute(path, map[string]string{}, true)
		if err != nil {
			continue
		}

		logger.Info("shut down deployment",
			zap.String("target", target.String()))
	}
}

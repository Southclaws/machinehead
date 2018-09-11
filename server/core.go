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

	"github.com/Southclaws/machinehead/types"
)

// App stores application state
type App struct {
	Config      Config
	Targets     map[string]types.Target
	GlobalEnvs  map[string]string
	Watcher     *gitwatch.Session
	SelfWatcher *gitwatch.Session
	Vault       *api.Client
	Auth        transport.AuthMethod

	L *zap.Logger

	ctx context.Context
	cf  context.CancelFunc
}

var (
	ErrExistingDaemon = errors.New("an existing instance of machinehead is already running here")
)

// Initialise creates a new instance and prepares it for starting
func Initialise(config Config, logger *zap.Logger) (app *App, err error) {
	exists := types.SocketExists()
	if exists {
		return nil, ErrExistingDaemon
	}

	ctx, cf := context.WithCancel(context.Background())

	app = &App{
		Config:  config,
		Targets: make(map[string]types.Target),
		L:       logger,
		ctx:     ctx,
		cf:      cf,
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
		vaultConfig := api.DefaultConfig()
		vaultConfig.Address = config.VaultAddress
		app.Vault, err = api.NewClient(vaultConfig)
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

	return
}

// Run will run the application and block until graceful exit like `app.Start`
// but this function returns an explicit error. This is for use when Machinehead
// is being used as a library instead of a command line application.
func (app *App) Run() (err error) {
	app.L.Debug("starting machinehead with debug logging",
		zap.Any("config", app.Config))

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

	app.L.Debug("done initial docker-compose up of targets")

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

	app.L.Debug("graceful shutdown initiated")

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

		app.L.Info("shut down deployment",
			zap.String("target", target.String()))
	}
}

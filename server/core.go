package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

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
func (app *App) Start() {
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
	}
}

func (app *App) start(errChan chan error) (err error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Kill, os.Interrupt)

	f := func() (errInner error) {
		select {
		case sig := <-c:
			return errors.New(sig.String())

		case errInner = <-errChan:
			return errors.Wrap(errInner, "git watcher encountered an error")

		case event := <-app.Watcher.Events:
			logger.Debug("event received",
				zap.String("path", event.Path),
				zap.String("repo", event.URL),
				zap.Time("timestamp", event.Timestamp))

			env, errInner := app.envForRepo(event.Path)
			if errInner != nil {
				return errors.Wrap(errInner, "failed to get secrets for project")
			}

			errInner = compose(event.Path, env, "up", "-d")
			if errInner != nil {
				return errors.Wrap(errInner, "failed to execute compose")
			}
		}
		return
	}

	for {
		err = f()
		if err != nil {
			break
		}
	}
	return err
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

// doInitialUp performs an initial `docker-compose up` for each target
func (app *App) doInitialUp() (err error) {
	var path string
	for _, target := range app.Config.Targets {
		path, err = gitwatch.GetRepoPath(app.Config.CacheDirectory, target)
		if err != nil {
			return errors.Wrap(err, "failed to get cached repository path")
		}

		var env map[string]string
		env, err = app.envForRepo(path)
		if err != nil {
			return errors.Wrap(err, "failed to get secrets for project")
		}

		err = compose(path, env, "up", "-d")
		if err != nil {
			return
		}
	}
	return
}

// envForRepo gets a set of environment variables for a given repo
func (app *App) envForRepo(path string) (result map[string]string, err error) {
	projectName := filepath.Base(path)
	secret, err := app.Vault.Logical().List(projectName)
	if err != nil {
		return
	}
	if secret == nil {
		return
	}

	result = make(map[string]string)
	var ok bool
	for k, v := range secret.Data {
		result[k], ok = v.(string)
		if !ok {
			continue
		}
	}

	return
}

func compose(path string, env map[string]string, command ...string) (err error) {
	logger.Info("running compose command",
		zap.Any("env", env),
		zap.Strings("args", command))

	cmd := exec.Command("docker-compose", command...)
	cmd.Dir = path
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	err = cmd.Run()
	return errors.Wrap(err, "failed to execute compose")
}

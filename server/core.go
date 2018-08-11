package server

import (
	"context"
	"os"
	"os/exec"
	"os/signal"

	"go.uber.org/zap"

	"github.com/pkg/errors"

	"github.com/Southclaws/machinehead/gitwatch"
)

// App stores application state
type App struct {
	Config  Config
	Watcher *gitwatch.Session

	ctx context.Context
	cf  context.CancelFunc
}

// Initialise creates a new instance and prepares it for starting
func Initialise(config Config) (app *App, err error) {
	ctx, cf := context.WithCancel(context.Background())
	gw, err := gitwatch.New(ctx, config.Targets, config.CheckInterval, config.CacheDirectory, true)
	if err != nil {
		cf()
		err = errors.Wrap(err, "failed to construct new git watcher")
		return
	}

	logger.Debug("starting machinehead with debug logging",
		zap.Any("config", config))

	app = &App{
		Config:  config,
		Watcher: gw,
		ctx:     ctx,
		cf:      cf,
	}

	return
}

// Start will start the application and block until graceful exit or fatal error
func (app *App) Start() (err error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Kill, os.Interrupt)

	f := func() (err error) {
		select {
		case sig := <-c:
			err = errors.New(sig.String())
			return

		case event := <-app.Watcher.Events:
			logger.Debug("event received",
				zap.String("path", event.Path),
				zap.String("repo", event.URL),
				zap.Time("timestamp", event.Timestamp))

			err = compose(event.Path, "up", "-d")
			if err != nil {
				logger.Error("failed to execute compose", zap.Error(err))
				err = nil
			}
		}
		return
	}

	// do an initial `docker-compose up` for each target
	var path string
	for _, target := range app.Config.Targets {
		path, err = gitwatch.GetRepoPath(app.Config.CacheDirectory, target)
		if err != nil {
			return
		}
		err = compose(path, "up", "-d")
		if err != nil {
			return
		}
	}

	for {
		err = f()
		if err != nil {
			break
		}
	}

	app.Stop()
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
		err = compose(path, "down")
		if err != nil {
			continue
		}

		logger.Info("shut down deployment",
			zap.String("target", target))
	}
}

func compose(path string, command ...string) (err error) {
	cmd := exec.Command("docker-compose", command...)
	cmd.Dir = path
	err = cmd.Run()
	return
}

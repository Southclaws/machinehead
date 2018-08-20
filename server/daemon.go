package server

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/Southclaws/machinehead/gitwatch"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// start runs the daemon and blocks until exit, it returns an error for
// `app.Start` to handle and log.
func (app *App) start() (err error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Kill, os.Interrupt)

	f := func() (errInner error) {
		select {
		case <-app.ctx.Done():
			logger.Debug("application internally terminated", zap.Error(app.ctx.Err()))
			return app.ctx.Err()

		case sig := <-c:
			return errors.New(sig.String())

		case errInner = <-app.Watcher.Errors:
			logger.Error("git watcher encountered an error",
				zap.Error(errInner))

		case event := <-app.Watcher.Events:
			logger.Debug("event received",
				zap.String("path", event.Path),
				zap.String("repo", event.URL),
				zap.Time("timestamp", event.Timestamp))

			env, errInner := app.envForRepo(event.Path)
			if errInner != nil {
				logger.Error("failed to get secrets for project",
					zap.Error(errInner))
			}

			errInner = compose(event.Path, env, "up", "-d")
			if errInner != nil {
				logger.Error("failed to execute compose",
					zap.Error(errInner))
			}
		}
		return nil
	}

	logger.Debug("starting background daemon")

	for {
		err = f()
		if err != nil {
			return
		}
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

// compose runs a docker-compose command with the given environment variables
func compose(path string, env map[string]string, command ...string) (err error) {
	logger.Debug("running compose command",
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

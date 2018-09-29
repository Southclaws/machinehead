package server

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/Southclaws/gitwatch"
	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-git.v4"
)

// sets up the gitwatch daemon, called during initialisation and at runtime
// whenever the config is updated to add new targets to the watcher.
func (app *App) setupGitWatcher() (err error) {
	if app.Watcher != nil {
		app.Watcher.Close()
	}
	app.Watcher, err = gitwatch.New(
		app.ctx,
		app.Config.Targets,
		time.Duration(app.Config.CheckInterval),
		app.Config.CacheDirectory,
		app.Auth,
		true,
	)
	if err != nil {
		err = errors.Wrap(err, "failed to construct new git watcher")
		return
	}
	return
}

func (app *App) setupSelfRepoWatcher() (session *gitwatch.Session, err error) {
	wd, err := os.Getwd()
	if err != nil {
		err = errors.Wrap(err, "failed to get working directory")
		return
	}

	repo, err := git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		err = errors.Wrapf(err, "failed to open %s as repository", wd)
		return
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		err = errors.Wrap(err, "failed to get remote 'origin'")
		return
	}

	session, err = gitwatch.New(
		app.ctx,
		[]string{remote.Config().URLs[0]},
		time.Duration(app.Config.CheckInterval),
		app.Config.CacheDirectory,
		app.Auth,
		true,
	)
	if err != nil {
		err = errors.Wrap(err, "failed to create git watcher")
		return
	}

	return
}

// start runs the daemon and blocks until exit, it returns an error for
// `app.Start` to handle and log.
func (app *App) start() (err error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Kill, os.Interrupt)

	configWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrap(err, "failed to create configuration file watcher")
	}

	err = configWatcher.Add("machinehead.json")
	if err != nil {
		return errors.Wrap(err, "failed to add machinehead.json to file watcher")
	}

	app.SelfWatcher, err = app.setupSelfRepoWatcher()
	if err != nil {
		return errors.Wrap(err, "failed to create current directory git watcher")
	}

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

		case errInner = <-configWatcher.Errors:
			logger.Error("config watcher encountered an error",
				zap.Error(errInner))

		case errInner = <-app.SelfWatcher.Errors:
			logger.Error("self repo watcher encountered an error",
				zap.Error(errInner))

		case <-configWatcher.Events:
			errInner = app.setupGitWatcher()
			if errInner != nil {
				logger.Error("failed to re-create git watcher with new config",
					zap.Error(errInner))
			}

		case event := <-app.SelfWatcher.Events:
			logger.Debug("working repository that contains config updated",
				zap.String("url", event.URL),
				zap.String("path", event.Path),
				zap.Time("timestamp", event.Timestamp))

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
	result = app.GlobalEnvs

	if app.Vault != nil {
		var (
			projectName = filepath.Base(path)
			secret      *api.Secret
		)

		secret, err = app.Vault.Logical().List(projectName)
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
	}

	return
}

// compose runs a docker-compose command with the given environment variables
func compose(path string, env map[string]string, command ...string) (err error) {
	logger.Debug("running compose command",
		zap.Any("env", env),
		zap.Strings("args", command))

	outBuf := bytes.NewBuffer(nil)

	cmd := exec.Command("docker-compose", command...)
	cmd.Dir = path
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	err = cmd.Run()
	if err != nil {
		err = errors.Wrap(err, outBuf.String())
	}
	return
}

package server

import (
	"os"
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

	var targets = make([]string, len(app.Config.Targets))
	for i, t := range app.Config.Targets {
		targets[i] = t.RepoURL
	}

	app.Watcher, err = gitwatch.New(
		app.ctx,
		targets,
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
		return
	}

	repo, err := git.PlainOpen(wd)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			return nil, nil
		}
		return
	}

	remote, err := repo.Remote("origin")
	if err != nil {
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

		// Halt and error signals

		case <-app.ctx.Done():
			app.L.Debug("application internally terminated", zap.Error(app.ctx.Err()))
			return app.ctx.Err()

		case sig := <-c:
			return errors.New(sig.String())

		case errInner = <-app.Watcher.Errors:
			app.L.Error("git watcher encountered an error",
				zap.Error(errInner))

		case errInner = <-configWatcher.Errors:
			app.L.Error("config watcher encountered an error",
				zap.Error(errInner))

		// TODO: fix with channel joiner
		// case errInner = <-app.SelfWatcher.Errors:
		// 	app.L.Error("self repo watcher encountered an error",
		// 		zap.Error(errInner))

		// Operational events

		case <-configWatcher.Events:
			errInner = app.setupGitWatcher()
			if errInner != nil {
				app.L.Error("failed to re-create git watcher with new config",
					zap.Error(errInner))
			}

		// case event := <-app.SelfWatcher.Events:
		// 	app.L.Debug("working repository that contains config updated",
		// 		zap.String("url", event.URL),
		// 		zap.String("path", event.Path),
		// 		zap.Time("timestamp", event.Timestamp))

		case event := <-app.Watcher.Events:
			app.L.Debug("event received",
				zap.String("path", event.Path),
				zap.String("repo", event.URL),
				zap.Time("timestamp", event.Timestamp))

			env, errInner := app.envForRepo(event.Path)
			if errInner != nil {
				app.L.Error("failed to get secrets for project",
					zap.String("target", event.URL),
					zap.Error(errInner))
			}

			target := app.Targets[event.URL]

			errInner = target.Execute(event.Path, env, false)
			if errInner != nil {
				app.L.Error("failed to execute compose",
					zap.Error(errInner))
			}
		}
		return nil
	}

	app.L.Debug("starting background daemon")

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
		path, err = gitwatch.GetRepoPath(app.Config.CacheDirectory, target.RepoURL)
		if err != nil {
			return errors.Wrapf(err, "failed to get cached repository path for %s", target.String())
		}

		var env map[string]string
		env, err = app.envForRepo(path)
		if err != nil {
			return errors.Wrapf(err, "failed to get secrets for %s", target.String())
		}

		err = target.Execute(path, env, false)
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

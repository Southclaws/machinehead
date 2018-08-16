// Package gitwatch provides a simple tool to first clone a set of git
// repositories to a local directory and then periodically check them all for
// any updates.
package gitwatch

import (
	"context"
	"net/url"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
)

// Session represents a git watch session configuration
type Session struct {
	Repositories []string      // list of local or remote repository URLs to watch
	Interval     time.Duration // the interval between remote checks
	Directory    string        // the directory to store repositories
	InitialEvent bool          // if true, an event for each repo will be emitted upon construction
	InitialDone  chan struct{} // if InitialEvent true, this is pushed to after initial setup done
	Events       chan Event    // when a change is detected, events are pushed here

	ctx context.Context
	cf  context.CancelFunc
}

// Event represents an update detected on one of the watched repositories
type Event struct {
	URL       string
	Path      string
	Timestamp time.Time
}

// New constructs a new git watch session on the given repositories
func New(
	ctx context.Context,
	repos []string,
	interval time.Duration,
	dir string,
	initialEvent bool,
) (session *Session, err error) {
	ctx2, cf := context.WithCancel(ctx)
	session = &Session{
		Repositories: repos,
		Interval:     interval,
		Directory:    dir,
		Events:       make(chan Event),
		InitialEvent: initialEvent,
		InitialDone:  make(chan struct{}),

		ctx: ctx2,
		cf:  cf,
	}
	return
}

// Run begins the watcher and blocks until an error occurs
func (s *Session) Run() (err error) {
	return s.daemon()
}

// Close gracefully shuts down the git watcher
func (s *Session) Close() {
	s.cf()
}

func (s *Session) daemon() (err error) {
	t := time.NewTicker(s.Interval)

	f := func() (err error) {
		select {
		case <-s.ctx.Done():
			err = context.Canceled
			break
		case <-t.C:
			err := s.checkRepos()
			if err != nil {
				break
			}
		}
		return
	}

	if s.InitialEvent {
		err = s.checkRepos()
		if err != nil {
			return err
		}
		s.InitialDone <- struct{}{}
	}

	for {
		err = f()
		if err != nil {
			return err
		}
	}
}

func (s *Session) checkRepos() (err error) {
	for _, repoPath := range s.Repositories {
		var event *Event
		event, err = s.checkRepo(repoPath)
		if err != nil {
			return
		}

		if event != nil {
			go func() { s.Events <- *event }()
		}
	}
	return
}

func (s *Session) checkRepo(repoPath string) (event *Event, err error) {
	path, err := GetRepoPath(s.Directory, repoPath)
	if err != nil {
		err = errors.Wrap(err, "failed to get path from repo url")
		return
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		if err != git.ErrRepositoryNotExists {
			err = errors.Wrap(err, "failed to open local repo")
			return
		}

		repo, err = git.PlainClone(path, false, &git.CloneOptions{
			URL: repoPath,
		})
		if err != nil {
			err = errors.Wrap(err, "failed to clone initial copy of repository")
			return
		}

		if s.InitialEvent {
			event, err = eventFromRepo(repo)
		}
		return
	}

	return eventFromChanges(repo)
}

func eventFromChanges(repo *git.Repository) (event *Event, err error) {
	wt, err := repo.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get worktree")
	}

	err = wt.Pull(&git.PullOptions{})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to pull local repo")
	}

	return eventFromRepo(repo)
}

func eventFromRepo(repo *git.Repository) (event *Event, err error) {
	wt, err := repo.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get worktree")
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		return
	}
	ref, err := repo.Head()
	if err != nil {
		return
	}
	c, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return
	}
	return &Event{
		URL:       remote.Config().URLs[0],
		Path:      wt.Filesystem.Root(),
		Timestamp: c.Author.When,
	}, nil
}

// GetRepoPath returns the local path of a cached repo from the given cache
func GetRepoPath(cache, repo string) (path string, err error) {
	u, err := url.Parse(repo)
	if err != nil {
		return
	}
	return filepath.Join(cache, filepath.Base(u.Path)), nil
}

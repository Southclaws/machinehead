package server

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/libcompose/config"
	"github.com/docker/libcompose/project"
	"github.com/stretchr/testify/assert"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/yaml.v2"
)

const repositories = "./test/repositories"
const cache = "./test/cache"

var app *App

func TestMain(m *testing.M) {
	close, err := setup()
	defer close()
	if err != nil {
		close()
		log.Fatal(err)
	}

	ret := make(chan error, 1)
	go func() {
		log.Println("Starting application concurrently")
		ret <- app.Run()
	}()
	log.Println("Waiting 5 seconds for initial up")
	time.Sleep(time.Second * 5)
	log.Println("Running tests")

	result := m.Run()
	log.Println("Stopping app")
	app.Stop()

	log.Println("Waiting for return from app.Run()")
	err = <-ret
	if err != nil && err != context.Canceled {
		log.Fatal("Application exited with error:", err)
	}

	os.Exit(result)
}

func TestUpdateOne(t *testing.T) {
	if err := commitTestRepo("one", []string{"sleep", "1000"}); err != nil {
		t.Error(err)
		return
	}
	// wait for at least one daemon cycle tick
	time.Sleep(time.Second * 30)

	w := bytes.NewBuffer(nil)
	cmd := exec.Command("docker-compose", "top")
	cmd.Dir = filepath.Join(cache, "one")
	cmd.Stdout = w
	err := cmd.Run()
	if err != nil {
		t.Error(err)
		return
	}

	lines := strings.Split(w.String(), "\n")
	assert.Regexp(t, `([0-9]+)\s+(\w+)\s+([0-9:]+)\s+sleep 1000`, lines[3])
}

func setup() (close context.CancelFunc, err error) {
	// delete the cache and repos dir from previous tests
	if err = (os.RemoveAll(repositories)); err != nil {
		return
	}
	if err = (os.RemoveAll(cache)); err != nil {
		return
	}
	// create the repos dir for mock remote repos
	if err = (os.MkdirAll(repositories, 0700)); err != nil {
		return
	}
	// create the mock repos, in reaility these would liekly be remote
	if err = (createTestRepo("one")); err != nil {
		return
	}
	if err = (createTestRepo("two")); err != nil {
		return
	}
	if err = (createTestRepo("three")); err != nil {
		return
	}

	vaultCtx, vaultClose := context.WithCancel(context.Background())
	close = func() {
		fmt.Println("closing vault instance")
		vaultClose()
	}
	if err = startVault(vaultCtx); err != nil {
		return
	}
	// wait for vault to spin up
	time.Sleep(time.Second)

	app, err = Initialise(Config{
		Targets: []string{
			filepath.Join(repositories, "one"),
			filepath.Join(repositories, "two"),
			filepath.Join(repositories, "three"),
		},
		CheckInterval:  time.Second,
		CacheDirectory: cache,
		VaultAddress:   "http://127.0.0.1:8200",
		VaultToken:     "1234",
		// VaultNamespace: "",
	})
	if err != nil {
		return
	}

	s, err := app.Vault.Logical().Write("secret/test", map[string]interface{}{"key1": "value1", "key2": "value2"})
	if err != nil {
		return
	}
	fmt.Println(s)

	return
}

func createTestRepo(name string) (err error) {
	path := filepath.Join(repositories, name)
	repo, err := git.PlainInit(path, false)
	if err != nil {
		return
	}
	wt, err := repo.Worktree()
	if err != nil {
		return
	}
	err = writeDC(path, []string{"sleep", "9999"})
	if err != nil {
		return
	}
	_, err = wt.Add("docker-compose.yml")
	if err != nil {
		return
	}
	_, err = wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{
		Name:  "test",
		Email: "test@test.com",
		When:  time.Now(),
	}})
	if err != nil {
		return
	}
	return
}

func commitTestRepo(name string, command []string) (err error) {
	path := filepath.Join(repositories, name)
	repo, err := git.PlainOpen(path)
	if err != nil {
		return
	}
	wt, err := repo.Worktree()
	if err != nil {
		return
	}
	err = writeDC(path, command)
	if err != nil {
		return
	}
	_, err = wt.Add("docker-compose.yml")
	if err != nil {
		return
	}
	_, err = wt.Commit("new", &git.CommitOptions{Author: &object.Signature{
		Name:  "test",
		Email: "test@test.com",
		When:  time.Now(),
	}})
	if err != nil {
		return
	}
	return
}

func writeDC(path string, command []string) (err error) {
	dc := project.ExportedConfig{
		Version: "3",
		Services: map[string]*config.ServiceConfig{
			"box": {
				Image:   "busybox",
				Command: command,
			},
		},
	}
	b, err := yaml.Marshal(dc)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(filepath.Join(path, "docker-compose.yml"), b, 0700)
	if err != nil {
		return
	}
	return
}

func startVault(ctx context.Context) (err error) {
	ch := make(chan error, 1)
	go func(c chan error) {
		cmd := exec.CommandContext(ctx, "vault", "server", "-dev", "-dev-root-token-id=1234")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		e := cmd.Run()
		if c != nil {
			c <- e
		}
	}(ch)

	t := time.NewTimer(time.Second)
	select {
	case <-t.C:
		return nil
	case err := <-ch:
		return err
	}
}

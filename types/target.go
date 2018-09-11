package types

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
)

// Target represents a git repository with a set of tasks to perform each time
// that repository receives a new commit.
type Target struct {
	// An optional label for the target
	Name string `json:"name"`

	// The repository URL to watch for changes, either http or ssh.
	RepoURL string `required:"true" json:"url"`

	// The command to run on each new Git commit
	Command []string `required:"true" json:"command"`

	// Environment variables associated with the target - do not store credentials here!
	Env map[string]string `json:"env"`

	// Write an .env file to the repo directory for testing and debug purposes
	WriteEnv bool `json:"write_env"`

	// Whether or not to run `Command` on first run, useful if the command is `docker-compose up`
	InitialRun bool `json:"initial_run"`

	// ShutdownCommand specifies the command to run during a graceful shutdown of Machinehead
	ShutdownCommand []string `json:"shutdown_command"`
}

// String returns the name of the target if present, otherwise the target URL
func (t Target) String() string {
	if t.Name == "" {
		return t.RepoURL
	}
	return t.Name
}

// Execute runs the target's command in the specified directory with the
// specified environment variables
func (t *Target) Execute(dir string, env map[string]string, shutdown bool) (err error) {
	for k, v := range t.Env {
		env[k] = v
	}

	var command []string
	if shutdown {
		command = t.ShutdownCommand
	} else {
		command = t.Command
	}

	return execute(dir, env, command)
}

func execute(dir string, env map[string]string, command []string) (err error) {
	if len(command) == 0 {
		return
	}

	outBuf := bytes.NewBuffer(nil)

	cmd := exec.Command(command[0])
	if len(command) > 1 {
		cmd.Args = append(cmd.Args, command[1:]...)
	}
	cmd.Dir = dir
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf

	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, outBuf.String())
	}

	return
}

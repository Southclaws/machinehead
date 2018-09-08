package server

// Target represents a git repository with a set of tasks to perform each time
// that repository receives a new commit.
type Target struct {
	// An optional label for the target
	Name string `required:"false" json:"name"`

	// The repository URL to watch for changes, either http or ssh.
	RepoURL string `required:"true" json:"repo_url"`

	// The command to run on each new Git commit
	Command []string `required:"true" json:"command"`

	// Environment variables associated with the target - do not store credentials here!
	Env map[string]string `required:"false" json:"env"`

	// Write an .env file to the repo directory for testing and debug purposes
	WriteEnv bool `required:"false" json:"write_env"`
}

func (t Target) String() string {
	if t.Name == "" {
		return t.RepoURL
	}
	return t.Name
}

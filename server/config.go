package server

import (
	"time"
)

// Config defines configuration fields
type Config struct {
	Targets        []string      `split_words:"true" required:"true"`
	CheckInterval  time.Duration `split_words:"true" required:"true"`
	CacheDirectory string        `split_words:"true" required:"true"`
	VaultAddress   string        `split_words:"true" required:"true"`
	VaultToken     string        `split_words:"true" required:"true"`
	VaultNamespace string        `split_words:"true" default:"machinehead"`
}

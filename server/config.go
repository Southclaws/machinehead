package server

import (
	"time"
)

// Config defines configuration fields
type Config struct {
	Targets        []string      `required:"true"`
	CheckInterval  time.Duration `required:"true"`
	CacheDirectory string        `required:"true"`
}

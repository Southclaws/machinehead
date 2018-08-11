package server

import (
	"os"

	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	config := zap.NewProductionConfig()
	if os.Getenv("DEBUG") != "" {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	// nolint
	logger, _ = config.Build()
}

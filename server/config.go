package server

import (
	"encoding/json"
	"time"

	"github.com/Southclaws/configor"
	"github.com/pkg/errors"
)

// Config defines configuration fields
type Config struct {
	Targets        []Target `required:"true"   json:"targets"`
	CheckInterval  Duration `default:"1s"      json:"check_interval"`
	CacheDirectory string   `default:"./cache" json:"cache_directory"`
	VaultAddress   string   `required:"false"  json:"vault_address"`
	VaultToken     string   `required:"false"  json:"vault_token"`
	VaultNamespace string   `required:"false"  json:"vault_namespace"`
}

// LoadConfig reads configuration from the current working directory
func LoadConfig() (config Config, err error) {
	err = configor.Load(&config, "machinehead.json")
	if err != nil {
		return
	}
	return
}

// Duration wraps time.Duration to solve config unmarshalling
type Duration time.Duration

// MarshalJSON implements JSON marshalling for durations
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON implements JSON unmarshalling for durations
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		duration, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(duration)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

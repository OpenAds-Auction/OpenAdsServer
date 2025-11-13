package signatures

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prebid/prebid-server/v3/util/jsonutil"
)

const (
	SchemaVersion = 1
)

type TransportType string

const (
	TransportUDS TransportType = "uds"
	TransportTCP TransportType = "tcp"
)

type Config struct {
	Enabled         bool          `json:"enabled"`
	Transport       TransportType `json:"transport"`
	BasePath        string        `json:"base_path"`
	RequestPath     string        `json:"request_path"`
	RejectOnFailure bool          `json:"reject_on_failure"`
	Version         int           `json:"-"`
}

func NewConfig(rawConfig json.RawMessage) (*Config, error) {
	cfg := &Config{}

	if err := jsonutil.UnmarshalValid(rawConfig, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.Version = SchemaVersion

	if cfg.Transport != TransportUDS && cfg.Transport != TransportTCP {
		return nil, fmt.Errorf("invalid transport: %s (must be 'uds' or 'tcp')", cfg.Transport)
	}

	if cfg.BasePath == "" {
		return nil, fmt.Errorf("base_path is required")
	}

	if cfg.RequestPath == "" {
		return nil, fmt.Errorf("request_path is required")
	}

	cfg.BasePath = strings.TrimRight(cfg.BasePath, "/")
	cfg.RequestPath = strings.TrimLeft(cfg.RequestPath, "/")

	return cfg, nil
}

package ea_azure

import (
	"errors"
	"time"

	"github.com/elastic/elastic-agent-libs/transport/httpcommon"
)

const (
	// The default incremental update interval.
	defaultUpdateInterval = time.Minute * 15
	// The default full synchronization interval.
	defaultSyncInterval = time.Hour * 24
)

// conf contains parameters needed to configure the input.
type conf struct {
	ClientID       string                           `config:"client_id" validate:"required"`
	TenantID       string                           `config:"tenant_id" validate:"required"`
	Secret         string                           `config:"secret" validate:"required"`
	LoginURL       string                           `config:"login_url"`
	LoginScopes    []string                         `config:"login_scopes"`
	SyncInterval   time.Duration                    `config:"sync_interval"`
	UpdateInterval time.Duration                    `config:"update_interval"`
	Transport      httpcommon.HTTPTransportSettings `config:",inline"`
}

// Validate runs validation against the config.
func (c *conf) Validate() error {
	if c.SyncInterval < c.UpdateInterval {
		return errors.New("sync_interval must be longer than update_interval")
	}

	return nil
}

// defaultConfig returns a default configuration.
func defaultConf() conf {
	return conf{
		SyncInterval:   defaultSyncInterval,
		UpdateInterval: defaultUpdateInterval,
	}
}

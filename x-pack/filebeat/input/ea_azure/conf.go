package ea_azure

import (
	"errors"
	"time"
)

const (
	// The default incremental update interval.
	defaultUpdateInterval = time.Minute * 15
	// The default full synchronization interval.
	defaultSyncInterval = time.Hour * 24
)

// conf contains parameters needed to configure the input.
type conf struct {
	TenantID       string        `config:"tenant_id" validate:"required"`
	SyncInterval   time.Duration `config:"sync_interval"`
	UpdateInterval time.Duration `config:"update_interval"`
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

package identity

import (
	"fmt"

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-concert/unison"

	"github.com/elastic/beats/v7/filebeat/input/v2"
	"github.com/elastic/beats/v7/libbeat/feature"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider"

	_ "github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/azure"
	_ "github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/okta"
)

const Name = "identity"

func Plugin(logger *logp.Logger) v2.Plugin {
	return v2.Plugin{
		Name:      Name,
		Stability: feature.Experimental,
		Info:      "Identity Provider for Entity Analytics",
		Doc:       "Collect identity assets for Entity Analytics",
		Manager: &manager{
			logger: logger,
		},
	}
}

var _ v2.InputManager = &manager{}

type manager struct {
	logger   *logp.Logger
	provider provider.Provider
}

func (m *manager) Init(grp unison.Group, mode v2.Mode) error {
	return m.provider.Init(grp, mode)
}

func (m *manager) Create(cfg *config.C) (v2.Input, error) {
	var c conf
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("unable to unpack %s input config: %w", Name, err)
	}

	factoryFn, err := provider.Get(c.Provider)
	if err != nil {
		return nil, fmt.Errorf("unable to create %s input: %w", Name, err)
	}

	m.provider, err = factoryFn(m.logger)
	if err != nil {
		return nil, fmt.Errorf("unable to create %s input provider: %w", Name, err)
	}

	return m.provider.Create(cfg)
}

package kvstore

import (
	"errors"

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-concert/unison"

	"github.com/elastic/beats/v7/filebeat/input/v2"
)

// ErrNoSourcesConfigured indicates no sources were given in the configuration.
var ErrNoSourcesConfigured = errors.New("no sources configured")

// Manager is used to create, manage, and coordinate inputs which use a key/value
// store for their persistent state.
type Manager struct {
	// Logger for writing log messages.
	Logger *logp.Logger

	// Type must contain the name of the input type.
	Type string

	// Configure returns a configured Input instance and a slice of Sources
	// that will be used to collect events.
	Configure func(cfg *config.C) (Input, []Source, error)
}

type managerConfig struct {
	ID string `config:"id" validate:"required"`
}

// Init initializes any required resources. It is currently a no-op, key/value
// stores are managed per-source for an input.
func (m *Manager) Init(grp unison.Group, mode v2.Mode) error {
	return nil
}

// Create makes a new v2.Input using the provided config.C which will be
// used in the Manager's Configure function.
func (m *Manager) Create(c *config.C) (v2.Input, error) {
	inp, sources, err := m.Configure(c)
	if err != nil {
		return nil, err
	}

	settings := managerConfig{}
	if err = c.Unpack(&settings); err != nil {
		return nil, err
	}

	if len(sources) == 0 {
		return nil, ErrNoSourcesConfigured
	}

	return &input{
		id:           settings.ID,
		manager:      m,
		sources:      sources,
		managedInput: inp,
	}, nil
}

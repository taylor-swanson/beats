package kvstore

import (
	"errors"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-concert/unison"

	v2 "github.com/elastic/beats/v7/filebeat/input/v2"
)

var ErrNoSourcesConfigured = errors.New("no sources configured")
var ErrNoInputRunner = errors.New("no input runner")

var _ v2.InputManager = &Manager{}

type Manager struct {
	Logger    *logp.Logger
	Type      string
	Configure func(cfg *config.C) (Input, []Source, error)
}

func (m *Manager) Init(grp unison.Group, mode v2.Mode) error {
	return nil
}

func (m *Manager) Create(c *config.C) (v2.Input, error) {
	inp, sources, err := m.Configure(c)
	if err != nil {
		return nil, err
	}

	settings := struct {
		ID string `config:"id"`
	}{}
	if err = c.Unpack(&settings); err != nil {
		return nil, err
	}

	if len(sources) == 0 {
		return nil, ErrNoSourcesConfigured
	}
	if inp == nil {
		return nil, ErrNoInputRunner
	}

	return &input{
		id:      settings.ID,
		manager: m,
		sources: sources,
		input:   inp,
	}, nil
}

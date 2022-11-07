package okta

import (
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/beats/v7/filebeat/input/v2"
	stateless "github.com/elastic/beats/v7/filebeat/input/v2/input-stateless"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider"
)

const Name = "okta"

// okta implements the provider.Provider interface.
var _ provider.Provider = &okta{}

type okta struct {
	*stateless.InputManager

	logger *logp.Logger
}

func (p *okta) Name() string {
	//TODO implement me
	panic("implement me")
}

func (p *okta) Test(context v2.TestContext) error {
	//TODO implement me
	panic("implement me")
}

func (p *okta) Run(ctx v2.Context, publish stateless.Publisher) error {
	//TODO implement me
	panic("implement me")
}

func (p *okta) configure(cfg *config.C) (stateless.Input, error) {
	return p, nil
}

func New(logger *logp.Logger) (provider.Provider, error) {
	p := okta{
		logger: logger,
	}
	p.InputManager = &stateless.InputManager{
		Configure: p.configure,
	}
	return &p, nil // TODO
}

func init() {
	if err := provider.Register(Name, New); err != nil {
		panic(err)
	}
}

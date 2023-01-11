package mock

import (
	"context"
	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/azure/authenticator"
)

const DefaultTokenValue = "test-token"

type mock struct {
	tokenValue string
}

func (a *mock) Token(ctx context.Context) (string, error) {
	return a.tokenValue, nil
}

func (a *mock) SetLogger(_ *logp.Logger) {}

func New(tokenValue string) authenticator.Authenticator {
	if tokenValue == "" {
		tokenValue = DefaultTokenValue
	}

	return &mock{tokenValue: tokenValue}
}

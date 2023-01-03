package mock

import (
	"context"

	"github.com/elastic/beats/v7/x-pack/filebeat/input/ea_azure/authenticator"
)

const DefaultTokenValue = "test-token"

type mock struct {
	tokenValue string
}

func (a *mock) Token(ctx context.Context) (string, error) {
	return a.tokenValue, nil
}

func New(tokenValue string) authenticator.Authenticator {
	if tokenValue == "" {
		tokenValue = DefaultTokenValue
	}

	return &mock{tokenValue: tokenValue}
}

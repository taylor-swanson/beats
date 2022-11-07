package authenticator

import (
	"context"

	"github.com/elastic/elastic-agent-libs/logp"
)

type Authenticator interface {
	Token(ctx context.Context) (string, error)
	SetLogger(logger *logp.Logger)
}

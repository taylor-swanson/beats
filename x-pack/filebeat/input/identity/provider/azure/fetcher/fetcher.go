package fetcher

import (
	"context"

	"github.com/elastic/elastic-agent-libs/logp"
)

type Fetcher interface {
	Groups(ctx context.Context, deltaLink string) ([]*Group, string, error)
	Users(ctx context.Context, deltaLink string) ([]*User, string, error)
	SetLogger(logger *logp.Logger)
}

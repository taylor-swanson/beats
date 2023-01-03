package fetcher

import "context"

type Fetcher interface {
	Groups(ctx context.Context, deltaLink string) ([]*Group, string, error)
	Users(ctx context.Context, deltaLink string) ([]*User, string, error)
}

package authenticator

import "context"

type Authenticator interface {
	Token(ctx context.Context) (string, error)
}

package ea_azure

import "golang.org/x/net/context"

const DefaultTokenValue = "test-token"

type authMock struct {
	tokenValue string
}

func (a *authMock) Token(ctx context.Context) (string, error) {
	return a.tokenValue, nil
}

func newAuthMock(tokenValue string) Authenticator {
	if tokenValue == "" {
		tokenValue = DefaultTokenValue
	}

	return &authMock{tokenValue: tokenValue}
}

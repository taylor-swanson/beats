package mock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMock_Token(t *testing.T) {
	tests := map[string]struct {
		InToken string
		Want    string
	}{
		"default": {
			InToken: "",
			Want:    DefaultTokenValue,
		},
		"user-defined": {
			InToken: "some-value",
			Want:    "some-value",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := New(tc.InToken)

			got, gotErr := a.Token(context.Background())

			assert.NoError(t, gotErr)
			assert.Equal(t, tc.Want, got)
		})
	}
}

package provider

import (
	"errors"
	"testing"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/stretchr/testify/assert"
)

func TestRegistry(t *testing.T) {
	err := Register("test", func(logger *logp.Logger) (Provider, error) {
		return nil, errors.New("test error")
	})
	assert.NoError(t, err)
	err = Register("test", func(logger *logp.Logger) (Provider, error) {
		return nil, errors.New("test error")
	})
	assert.ErrorIs(t, err, ErrExists)

	exists := Has("test")
	assert.True(t, exists)
	exists = Has("foobar")
	assert.False(t, exists)

	_, err = Get("foobar")
	assert.ErrorIs(t, err, ErrNotFound)
	factoryFn, err := Get("test")
	assert.NoError(t, err)

	_, err = factoryFn(logp.L())
	assert.ErrorContains(t, err, "test error")
}

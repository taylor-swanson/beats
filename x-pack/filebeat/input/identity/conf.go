package identity

import (
	"errors"

	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider"
)

var (
	// ErrProviderUnknown is an error that indicates the provider type is not known.
	ErrProviderUnknown = errors.New("identity: unknown provider type")
)

type conf struct {
	Provider string `config:"provider" validate:"required"`
}

func (c *conf) Validate() error {
	if !provider.Has(c.Provider) {
		return ErrProviderUnknown
	}

	return nil
}

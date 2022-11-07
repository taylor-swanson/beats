package identity

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/azure"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/okta"
)

func TestConf_Validate(t *testing.T) {
	tests := map[string]struct {
		In      conf
		WantErr string
	}{
		"ok-provider-azure": {
			In: conf{
				Provider: azure.Name,
			},
		},
		"ok-provider-okta": {
			In: conf{
				Provider: okta.Name,
			},
		},
		"err-provider-unknown": {
			In: conf{
				Provider: "unknown",
			},
			WantErr: ErrProviderUnknown.Error(),
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.In.Validate()

			if tc.WantErr != "" {
				assert.ErrorContains(t, gotErr, tc.WantErr)
			} else {
				assert.NoError(t, gotErr)
			}
		})
	}
}

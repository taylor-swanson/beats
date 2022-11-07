package azure

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConf_Validate(t *testing.T) {
	tests := map[string]struct {
		In      conf
		WantErr string
	}{
		"default": {
			In:      defaultConf(),
			WantErr: "",
		},
		"err-invalid-intervals": {
			In: conf{
				SyncInterval:   time.Second,
				UpdateInterval: time.Second * 2,
			},
			WantErr: "sync_interval must be longer than update_interval",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.In.Validate()

			if tc.WantErr == "" {
				assert.NoError(t, gotErr)
			} else {
				assert.ErrorContains(t, gotErr, tc.WantErr)
			}
		})
	}
}

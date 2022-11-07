package collections

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

func TestMapStrAppendString(t *testing.T) {
	tests := map[string]struct {
		inM     mapstr.M
		inKey   string
		inValue string
		want    mapstr.M
	}{
		"empty": {
			inM:     mapstr.M{},
			inKey:   "foo",
			inValue: "bar",
			want:    mapstr.M{"foo": []string{"bar"}},
		},
		"existing-string": {
			inM:     mapstr.M{"foo": "one"},
			inKey:   "foo",
			inValue: "bar",
			want:    mapstr.M{"foo": []string{"one", "bar"}},
		},
		"existing-slice": {
			inM:     mapstr.M{"foo": []string{"one", "two"}},
			inKey:   "foo",
			inValue: "bar",
			want:    mapstr.M{"foo": []string{"one", "two", "bar"}},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			MapStrAppendString(tc.inM, tc.inKey, tc.inValue)

			assert.Equal(t, tc.want, tc.inM)
		})
	}
}

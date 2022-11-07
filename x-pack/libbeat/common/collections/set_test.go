package collections

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSet(t *testing.T) {
	tests := map[string]struct {
		In   []int
		Want *Set[int]
	}{
		"nil": {
			In:   nil,
			Want: &Set[int]{m: map[int]struct{}{}},
		},
		"empty": {
			In:   []int{},
			Want: &Set[int]{m: map[int]struct{}{}},
		},
		"1-elem": {
			In:   []int{1},
			Want: &Set[int]{m: map[int]struct{}{1: {}}},
		},
		"3-elem": {
			In:   []int{1, 2, 3},
			Want: &Set[int]{m: map[int]struct{}{1: {}, 2: {}, 3: {}}},
		},
		"dup-elem": {
			In:   []int{1, 2, 2},
			Want: &Set[int]{m: map[int]struct{}{1: {}, 2: {}}},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got := NewSet[int](tc.In...)
			assert.Equal(t, tc.Want, got)
		})
	}
}

func TestSet_UnmarshalJSON_Int(t *testing.T) {
	tests := map[string]struct {
		In      []byte
		Want    *Set[int]
		WantErr string
	}{
		"ok-nil": {
			In:   []byte(`null`),
			Want: NewSet[int](),
		},
		"ok-empty": {
			In:   []byte(`[]`),
			Want: NewSet[int](),
		},
		"ok-1-elem": {
			In:   []byte(`[0]`),
			Want: NewSet[int](0),
		},
		"ok-3-elem": {
			In:   []byte(`[0,1,2]`),
			Want: NewSet[int](0, 1, 2),
		},
		"ok-dup-elem": {
			In:   []byte(`[0,1,1]`),
			Want: NewSet[int](0, 1),
		},
		"err-mixed-types": {
			In:      []byte(`[0,"1",2]`),
			WantErr: "json: cannot unmarshal string into Go value of type int",
		},
		"err-not-list-str": {
			In:      []byte(`"1"`),
			WantErr: "json: cannot unmarshal string into Go value of type []int",
		},
		"err-not-list-int": {
			In:      []byte(`1`),
			WantErr: "json: cannot unmarshal number into Go value of type []int",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			s := NewSet[int]()

			gotErr := s.UnmarshalJSON(tc.In)

			if tc.WantErr != "" {
				assert.ErrorContains(t, gotErr, tc.WantErr)
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.Want, s)
			}
		})
	}
}

func TestSet_UnmarshalJSON_String(t *testing.T) {
	tests := map[string]struct {
		In      []byte
		Want    *Set[string]
		WantErr string
	}{
		"ok-nil": {
			In:   []byte(`null`),
			Want: NewSet[string](),
		},
		"ok-empty": {
			In:   []byte(`[]`),
			Want: NewSet[string](),
		},
		"ok-1-elem": {
			In:   []byte(`["alpha"]`),
			Want: NewSet[string]("alpha"),
		},
		"ok-3-elem": {
			In:   []byte(`["alpha", "beta", "gamma"]`),
			Want: NewSet[string]("alpha", "beta", "gamma"),
		},
		"ok-dup-elem": {
			In:   []byte(`["alpha", "beta", "beta"]`),
			Want: NewSet[string]("alpha", "beta"),
		},
		"err-mixed-types": {
			In:      []byte(`[0,"1",2]`),
			WantErr: "json: cannot unmarshal number into Go value of type string",
		},
		"err-not-list-str": {
			In:      []byte(`"1"`),
			WantErr: "json: cannot unmarshal string into Go value of type []string",
		},
		"err-not-list-int": {
			In:      []byte(`1`),
			WantErr: "json: cannot unmarshal number into Go value of type []string",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			s := NewSet[string]()

			gotErr := s.UnmarshalJSON(tc.In)

			if tc.WantErr != "" {
				assert.ErrorContains(t, gotErr, tc.WantErr)
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.Want, s)
			}
		})
	}
}

func TestSet_MarshalJSON_Int(t *testing.T) {
	tests := map[string]struct {
		In *Set[int]
		// Since Set doesn't define order of elements, the result will be
		// unmarshalled into an int slice, and assert.ElementsMatch will be
		// used to test equality (ignoring order of the elements).
		Want    []int
		WantErr string
	}{
		"ok-nil": {
			In:   nil,
			Want: nil,
		},
		"ok-empty": {
			In:   NewSet[int](),
			Want: nil,
		},
		"ok-1-elem": {
			In:   NewSet[int](0),
			Want: []int{0},
		},
		"ok-3-elem": {
			In:   NewSet[int](0, 1, 2),
			Want: []int{0, 1, 2},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var got []int

			gotRaw, gotErr := tc.In.MarshalJSON()

			err := json.Unmarshal(gotRaw, &got)
			assert.NoError(t, err)

			if tc.WantErr != "" {
				assert.ErrorContains(t, gotErr, tc.WantErr)
			} else {
				assert.NoError(t, gotErr)
				assert.ElementsMatch(t, tc.Want, got)
			}
		})
	}
}

func TestSet_MarshalJSON_String(t *testing.T) {
	tests := map[string]struct {
		In *Set[string]
		// Since Set doesn't define order of elements, the result will be
		// unmarshalled into a string slice, and assert.ElementsMatch will be
		// used to test equality (ignoring order of the elements).
		Want    []string
		WantErr string
	}{
		"ok-nil": {
			In:   nil,
			Want: nil,
		},
		"ok-empty": {
			In:   NewSet[string](),
			Want: nil,
		},
		"ok-1-elem": {
			In:   NewSet[string]("alpha"),
			Want: []string{"alpha"},
		},
		"ok-3-elem": {
			In:   NewSet[string]("alpha", "beta", "gamma"),
			Want: []string{"alpha", "beta", "gamma"},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var got []string

			gotRaw, gotErr := tc.In.MarshalJSON()

			err := json.Unmarshal(gotRaw, &got)
			assert.NoError(t, err)

			if tc.WantErr != "" {
				assert.ErrorContains(t, gotErr, tc.WantErr)
			} else {
				assert.NoError(t, gotErr)
				assert.ElementsMatch(t, tc.Want, got)
			}
		})
	}
}

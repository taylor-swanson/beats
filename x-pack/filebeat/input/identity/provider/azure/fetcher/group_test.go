package fetcher

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGroup_ToECS(t *testing.T) {
	in := Group{
		ID:   uuid.MustParse("88ecb4e8-5a1a-461e-a062-f1d3c5aa4ca4"),
		Name: "group1",
	}
	want := GroupECS{
		ID:   "88ecb4e8-5a1a-461e-a062-f1d3c5aa4ca4",
		Name: "group1",
	}

	got := in.ToECS()
	assert.Equal(t, want, got)
}

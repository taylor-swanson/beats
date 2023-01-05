package kvstore

import (
	"errors"
	v2 "github.com/elastic/beats/v7/filebeat/input/v2"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-concert/unison"
	"github.com/stretchr/testify/assert"

	"testing"
)

type testSource string

func (s testSource) Name() string {
	return string(s)
}

var _ Input = &testInput{}

type testInput struct{}

func (t testInput) Name() string {
	return "testInput"
}

func (t testInput) Test(testCtx v2.TestContext, source Source) error {
	return nil
}

func (t testInput) Run(inputCtx v2.Context, source Source, store *Store, client beat.Client) error {
	return nil
}

func configureOkay() func(cfg *config.C) (Input, []Source, error) {
	sources := []Source{
		testSource("test"),
	}

	return func(cfg *config.C) (Input, []Source, error) {
		return &testInput{}, sources, nil
	}
}

func configureErr() func(cfg *config.C) (Input, []Source, error) {
	return func(cfg *config.C) (Input, []Source, error) {
		return nil, nil, errors.New("test error")
	}
}

func configureNoSources() func(cfg *config.C) (Input, []Source, error) {
	return func(cfg *config.C) (Input, []Source, error) {
		return &testInput{}, nil, nil
	}
}

func TestManager_Create(t *testing.T) {
	t.Run("create-ok", func(t *testing.T) {
		m := Manager{
			Logger:    logp.L(),
			Type:      "test",
			Configure: configureOkay(),
		}

		c, err := config.NewConfigFrom(&managerConfig{ID: "create-ok"})
		assert.NoError(t, err)

		_, gotErr := m.Create(c)
		assert.NoError(t, gotErr)
	})

	t.Run("err-configure", func(t *testing.T) {
		m := Manager{
			Logger:    logp.L(),
			Type:      "test",
			Configure: configureErr(),
		}

		c, err := config.NewConfigFrom(&managerConfig{ID: "err-configure"})
		assert.NoError(t, err)

		_, gotErr := m.Create(c)
		assert.ErrorContains(t, gotErr, "test error")
	})

	t.Run("err-config-unpack", func(t *testing.T) {
		m := Manager{
			Logger:    logp.L(),
			Type:      "test",
			Configure: configureOkay(),
		}

		emptyCfg := struct{}{}
		c, err := config.NewConfigFrom(&emptyCfg)
		assert.NoError(t, err)

		_, gotErr := m.Create(c)
		assert.ErrorContains(t, gotErr, "string value is not set accessing 'id'")
	})

	t.Run("err-no-sources", func(t *testing.T) {
		m := Manager{
			Logger:    logp.L(),
			Type:      "test",
			Configure: configureNoSources(),
		}

		c, err := config.NewConfigFrom(&managerConfig{ID: "err-no-sources"})
		assert.NoError(t, err)

		_, gotErr := m.Create(c)
		assert.ErrorContains(t, gotErr, "no sources configured")
	})
}

func TestManager_Init(t *testing.T) {
	var grp unison.TaskGroup

	m := Manager{}
	gotErr := m.Init(&grp, v2.ModeRun)

	assert.NoError(t, gotErr)
}

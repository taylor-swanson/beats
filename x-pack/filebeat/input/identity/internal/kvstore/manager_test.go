package kvstore

import (
	"errors"
	"testing"

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-concert/unison"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/filebeat/input/v2"
)

func configureOkay() func(cfg *config.C) (Input, error) {
	return func(cfg *config.C) (Input, error) {
		return &testInput{}, nil
	}
}

func configureErr() func(cfg *config.C) (Input, error) {
	return func(cfg *config.C) (Input, error) {
		return nil, errors.New("test error")
	}
}

func configureNoSources() func(cfg *config.C) (Input, error) {
	return func(cfg *config.C) (Input, error) {
		return &testInput{}, nil
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
}

func TestManager_Init(t *testing.T) {
	var grp unison.TaskGroup

	m := Manager{}
	gotErr := m.Init(&grp, v2.ModeRun)

	assert.NoError(t, gotErr)
}

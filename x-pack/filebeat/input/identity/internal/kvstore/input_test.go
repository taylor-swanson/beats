package kvstore

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/paths"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/filebeat/input/v2"
	"github.com/elastic/beats/v7/libbeat/beat"
)

var _ Input = &testInput{}

type testInput struct {
	name   string
	testFn func(testCtx v2.TestContext) error
	runFn  func(runCtx v2.Context, store *Store, client beat.Client) error
}

func (t testInput) Name() string {
	return t.name
}

func (t testInput) Test(testCtx v2.TestContext) error {
	if t.testFn != nil {
		return t.testFn(testCtx)
	}

	return nil
}

func (t testInput) Run(inputCtx v2.Context, store *Store, client beat.Client) error {
	if t.runFn != nil {
		return t.runFn(inputCtx, store, client)
	}

	return nil
}

var _ beat.Pipeline = &testPipeline{}

type testPipeline struct {
}

func (t testPipeline) ConnectWith(_ beat.ClientConfig) (beat.Client, error) {
	return &testClient{}, nil
}

func (t testPipeline) Connect() (beat.Client, error) {
	return &testClient{}, nil
}

var _ beat.Client = &testClient{}

type testClient struct {
}

func (c *testClient) Publish(_ beat.Event) {

}

func (c *testClient) PublishAll(_ []beat.Event) {

}

func (c *testClient) Close() error {
	return nil
}

func TestInput_Name(t *testing.T) {
	name := "testInput"
	inp := input{
		managedInput: &testInput{
			name: name,
		},
	}

	assert.Equal(t, name, inp.Name())
}

func TestInput_Test(t *testing.T) {
	t.Run("test-ok", func(t *testing.T) {
		t.Parallel()

		called := false
		inp := input{
			managedInput: &testInput{
				testFn: func(testCtx v2.TestContext) error {
					called = true
					return nil
				},
			},
		}

		err := inp.Test(v2.TestContext{Logger: logp.L()})
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("test-err", func(t *testing.T) {
		t.Parallel()

		called := false
		inp := input{
			managedInput: &testInput{
				testFn: func(testCtx v2.TestContext) error {
					called = true

					return errors.New("test error")
				},
			},
		}

		err := inp.Test(v2.TestContext{Logger: logp.L()})
		assert.ErrorContains(t, err, "test error")
		assert.True(t, called)
	})

	t.Run("test-panic", func(t *testing.T) {
		t.Parallel()

		called := false
		inp := input{
			managedInput: &testInput{
				testFn: func(testCtx v2.TestContext) error {
					called = true

					panic("test panic")
				},
			},
		}

		err := inp.Test(v2.TestContext{Logger: logp.L()})
		assert.ErrorContains(t, err, "test panic")
		assert.True(t, called)
	})
}

func TestInput_Run(t *testing.T) {
	tmpDataDir, err := os.MkdirTemp(".", "test-input-run-*")
	if err != nil {
		panic(err)
	}

	paths.Paths = &paths.Path{Data: tmpDataDir}
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDataDir)
	})

	t.Run("run-ok", func(t *testing.T) {
		called := false
		inp := input{
			managedInput: &testInput{
				runFn: func(inputCtx v2.Context, store *Store, client beat.Client) error {
					called = true
					return nil
				},
			},
		}

		err = inp.Run(
			v2.Context{
				Logger:      logp.L(),
				ID:          "testInput",
				Cancelation: context.Background(),
			},
			&testPipeline{})

		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("run-err", func(t *testing.T) {
		called := false
		inp := input{
			managedInput: &testInput{
				runFn: func(inputCtx v2.Context, store *Store, client beat.Client) error {
					called = true
					return errors.New("test error")
				},
			},
		}

		err = inp.Run(
			v2.Context{
				Logger:      logp.L(),
				ID:          "testInput",
				Cancelation: context.Background(),
			},
			&testPipeline{})

		assert.ErrorContains(t, err, "test error")
		assert.True(t, called)
	})

	t.Run("run-panic", func(t *testing.T) {
		called := false
		inp := input{
			managedInput: &testInput{
				runFn: func(inputCtx v2.Context, store *Store, client beat.Client) error {
					called = true
					panic("test panic")
				},
			},
		}

		err = inp.Run(
			v2.Context{
				Logger:      logp.L(),
				ID:          "testInput",
				Cancelation: context.Background(),
			},
			&testPipeline{})

		assert.ErrorContains(t, err, "test panic")
		assert.True(t, called)
	})
}

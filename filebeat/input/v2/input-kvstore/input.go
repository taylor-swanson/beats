// Package kvstore provides a key/value store-based input. This is useful for
// inputs that need to persist a large amount of data for their state.
package kvstore

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/elastic/go-concert/ctxtool"
	"github.com/elastic/go-concert/unison"
	"github.com/urso/sderr"

	"github.com/elastic/beats/v7/filebeat/input/v2"
	"github.com/elastic/beats/v7/libbeat/beat"
)

type Input interface {
	Name() string
	Test(testCtx v2.TestContext, source Source) error
	Run(inputCtx v2.Context, source Source, store *Store, client beat.Client) error
}

var _ v2.Input = &input{}

type input struct {
	id           string
	manager      *Manager
	input        Input
	sources      []Source
	cleanTimeout time.Duration
}

func (n *input) Name() string {
	return n.input.Name()
}

func (n *input) Test(testCtx v2.TestContext) error {
	var grp unison.MultiErrGroup
	for _, source := range n.sources {
		source := source
		grp.Go(func() error {
			return n.testSource(testCtx, source)
		})
	}

	if errs := grp.Wait(); len(errs) > 0 {
		return sderr.WrapAll(errs, "input tests failed")
	}

	return nil
}

func (n *input) testSource(testCtx v2.TestContext, source Source) error {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("test input source panic with: %+v\n%s", r, debug.Stack())
			testCtx.Logger.Error("Input test panic: %+v", err)
		}
	}()

	return n.input.Test(testCtx, source)
}

func (n *input) Run(inputCtx v2.Context, connector beat.PipelineConnector) error {
	cancelCtx, cancel := context.WithCancel(ctxtool.FromCanceller(inputCtx.Cancelation))
	defer cancel()
	inputCtx.Cancelation = cancelCtx

	var grp unison.MultiErrGroup
	for _, source := range n.sources {
		source := source
		grp.Go(func() error {
			var err error

			sourceCtx := inputCtx
			sourceCtx.ID = inputCtx.ID + "_" + source.Name()
			sourceCtx.Logger = inputCtx.Logger.With("input_source", source.Name())

			if err = n.runSource(sourceCtx, source, connector); err != nil {
				cancel()
			}
			return err
		})
	}

	if errs := grp.Wait(); len(errs) > 0 {
		return sderr.WrapAll(errs, "input %s failed", inputCtx.ID)
	}

	inputCtx.Logger.Infof("ea_azure input exiting")

	return nil
}

func (n *input) runSource(inputCtx v2.Context, source Source, connector beat.PipelineConnector) error {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("input %s panic with: %+v\n%s", inputCtx.ID, r, debug.Stack())
			inputCtx.Logger.Errorf("Input %s panic: %+v", inputCtx.ID, err)
		}
	}()

	client, err := connector.ConnectWith(beat.ClientConfig{
		CloseRef:   inputCtx.Cancelation,
		ACKHandler: NewTxACKHandler(),
	})

	// TODO: Get data directory.
	store, err := NewStore(inputCtx.Logger, inputCtx.ID+".db", 0600)
	if err != nil {
		return err
	}
	defer store.Close()

	return n.input.Run(inputCtx, source, store, client)
}

// Package kvstore provides a key/value store-based input. This is useful for
// inputs that need to persist a large amount of data for their state.
package kvstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/elastic/elastic-agent-libs/paths"
	"github.com/elastic/go-concert/ctxtool"
	"github.com/elastic/go-concert/unison"
	"github.com/urso/sderr"

	"github.com/elastic/beats/v7/filebeat/input/v2"
	"github.com/elastic/beats/v7/libbeat/beat"
)

// Source describes a source the input can collect data from.
// The `Name` method must return a unique name that will be
// used to identify the source.
type Source interface {
	Name() string
}

// Input defines an interface for kvstore-based inputs.
type Input interface {
	// Name reports the input name.
	Name() string

	// Test runs the Test method for the configured source.
	Test(testCtx v2.TestContext, source Source) error

	// Run starts the data collection. Run must return an error only if the
	// error is fatal making it impossible for the input to recover.
	// The input run a go-routine can call Run per configured Source.
	Run(inputCtx v2.Context, source Source, store *Store, client beat.Client) error
}

var _ v2.Input = &input{}

type input struct {
	id           string
	manager      *Manager
	managedInput Input
	sources      []Source
	cleanTimeout time.Duration
}

// Name reports the input name.
func (n *input) Name() string {
	return n.managedInput.Name()
}

// Test runs the Test method for each configured source.
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

func (n *input) testSource(testCtx v2.TestContext, source Source) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("test input source panic with: %+v\n%s", r, debug.Stack())
			testCtx.Logger.Error("Input test panic: %+v", err)
		}
	}()

	return n.managedInput.Test(testCtx, source)
}

// Run creates a go-routine per source, waiting until all go-routines have
// returned, either by error, or by shutdown signal.
// If an input panics, we create an error value with stack trace to report the
// issue, but not crash the whole process.
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

func (n *input) runSource(inputCtx v2.Context, source Source, connector beat.PipelineConnector) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("input %s panic with: %+v\n%s", inputCtx.ID, r, debug.Stack())
			inputCtx.Logger.Errorf("Input %s panic: %+v", inputCtx.ID, err)
		}
	}()

	client, err := connector.ConnectWith(beat.ClientConfig{
		CloseRef:   inputCtx.Cancelation,
		ACKHandler: NewTxACKHandler(),
	})

	dataDir := paths.Resolve(paths.Data, "kvstore")
	if err = os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("kvstore: unable to make data directory: %w", err)
	}

	filename := filepath.Join(dataDir, inputCtx.ID+".db")
	store, err := NewStore(inputCtx.Logger, filename, 0600)
	if err != nil {
		return err
	}
	defer store.Close()

	return n.managedInput.Run(inputCtx, source, store, client)
}

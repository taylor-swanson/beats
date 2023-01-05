package kvstore

import (
	"context"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTxTracker_Ack(t *testing.T) {
}

func TestTxTracker_Add(t *testing.T) {
}

func TestTxTracker_Wait(t *testing.T) {
}

func TestTxACKHandler(t *testing.T) {
	t.Run("all-ack", func(t *testing.T) {
		txTracker := NewTxTracker(context.Background())
		handler := NewTxACKHandler()

		txTracker.Add()
		assert.Equal(t, 1, txTracker.pending.Load())

		handler.AddEvent(beat.Event{
			Private: txTracker,
		}, true)
		handler.ACKEvents(1)

		txTracker.Wait()

		assert.Zero(t, txTracker.pending.Load())
	})

	t.Run("wait-ack", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		txTracker := NewTxTracker(ctx)
		handler := NewTxACKHandler()

		txTracker.Add()
		assert.Equal(t, 1, txTracker.pending.Load())

		handler.AddEvent(beat.Event{
			Private: txTracker,
		}, true)

		txTracker.Wait()

		assert.Equal(t, 1, txTracker.pending.Load())
	})
}

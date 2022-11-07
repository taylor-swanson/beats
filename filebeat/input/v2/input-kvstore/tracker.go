package kvstore

import (
	"context"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/acker"
	"github.com/elastic/beats/v7/libbeat/common/atomic"
)

type TxTracker struct {
	pending atomic.Int
	ctx     context.Context
	cancel  context.CancelFunc
}

func (t *TxTracker) Add() {
	t.pending.Inc()
}

func (t *TxTracker) Ack() {
	if t.pending.Dec() == 0 {
		t.cancel()
	}
}

func (t *TxTracker) Wait() {
	if t.pending.Load() == 0 {
		t.cancel()
	}

	<-t.ctx.Done()
}

func NewTxTracker(ctx context.Context) *TxTracker {
	t := TxTracker{}
	t.ctx, t.cancel = context.WithCancel(ctx)

	return &t
}

func NewTxACKHandler() beat.ACKer {
	return acker.ConnectionOnly(acker.EventPrivateReporter(func(acked int, privates []interface{}) {
		for _, private := range privates {
			if t, ok := private.(*TxTracker); ok {
				t.Ack()
			}
		}
	}))
}

package kvstore

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

type encodeState struct {
	bytes.Buffer
	enc *gob.Encoder
}

func (e *encodeState) encode(value any) error {
	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)

	if encoder, ok := value.(Encoder); ok {
		if err := encoder.Encode(enc); err != nil {
			return fmt.Errorf("unable to encode value: %w", err)
		}
	} else {
		if err := e.enc.Encode(value); err != nil {
			return fmt.Errorf("unable to encode value: %w", err)
		}
	}

	return nil
}

func newEncodeState() *encodeState {
	es := encodeState{}
	es.enc = gob.NewEncoder(&es)

	return &es
}

// Encoder is the interface implemented by types that
// can encode a gob representation of themselves.
type Encoder interface {
	Encode(enc *gob.Encoder) error
}

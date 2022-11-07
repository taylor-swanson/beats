package kvstore

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
)

var decodeStatePool sync.Pool

// decodeState will decode bytes using a gob.Decoder.
type decodeState struct {
	bytes.Buffer
	dec *gob.Decoder
}

func (d *decodeState) decode(b []byte, value any) error {
	if _, err := d.Write(b); err != nil {
		return fmt.Errorf("unable to write data to decoder: %w", err)
	}

	if decoder, ok := value.(Decoder); ok {
		if err := decoder.Decode(d.dec); err != nil {
			return fmt.Errorf("unable to decode value: %w", err)
		}
	} else {
		if err := d.dec.Decode(value); err != nil {
			return fmt.Errorf("unable to decode value: %w", err)
		}
	}

	return nil
}

func newDecodeState() *decodeState {
	if v := decodeStatePool.Get(); v != nil {
		d := v.(*decodeState)
		d.Reset()

		return d
	}

	ds := decodeState{}
	ds.dec = gob.NewDecoder(&ds)

	return &ds
}

// Decoder is the interface implemented by types that
// can decode a gob-encoded representation of themselves.
type Decoder interface {
	Decode(dec *gob.Decoder) error
}

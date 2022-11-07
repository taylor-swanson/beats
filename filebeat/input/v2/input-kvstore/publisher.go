package kvstore

import (
	"github.com/elastic/beats/v7/filebeat/input/v2"
	"github.com/elastic/beats/v7/libbeat/beat"
)

type Publisher interface {
	Publish(event []beat.Event) error
}

type publisher struct {
	canceler v2.Canceler
	client   beat.Client
	store    *Store
}

func (p *publisher) Publish(events []beat.Event) error {
	p.client.PublishAll(events)

	return nil
}

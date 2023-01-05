package kvstore

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-agent-libs/logp"
)

func TestTransaction(t *testing.T) {
	writeTest := func(store *Store) error {
		tx, err := store.BeginTx(true)
		assert.NoError(t, err)
		defer func() {
			err := tx.Commit()
			assert.NoError(t, err)
		}()

		return nil
	}

	readTest := func(store *Store) error {
		tx, err := store.BeginTx(true)
		assert.NoError(t, err)
		defer func() {
			err := tx.Rollback()
			assert.NoError(t, err)
		}()

		return nil
	}

	dbFilename := "test-transaction.db"
	store, err := NewStore(logp.L(), dbFilename, 0644)
	assert.NoError(t, err)
	defer store.Close()

	t.Cleanup(func() {
		_ = os.Remove(dbFilename)
	})

	err = writeTest(store)
	assert.NoError(t, err)

	err = readTest(store)
	assert.NoError(t, err)
}

package kvstore

import (
	"fmt"
	"os"

	"github.com/elastic/elastic-agent-libs/logp"
	"go.etcd.io/bbolt"
)

type TxDoneFunc func(tx *Transaction, err error) error

// Store is a key/value store with transaction capabilities.
type Store struct {
	db     *bbolt.DB
	logger *logp.Logger
}

// RunTransaction runs a transaction. Multiple read-only transactions may be
// started, but only one write transaction is allowed at any given time.
//
// To close out a transaction, either commit the changes or rollback. For
// read-only transactions, no changes can be committed, so calling commit or
// rollback results in the transaction being "rolled back".
func (s *Store) RunTransaction(writable bool, fn func(tx *Transaction) error) (err error) {
	var t Transaction

	t.writeable = writable
	t.tx, err = s.db.Begin(writable)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = fmt.Errorf("kvstore transaction: recovered panic: %w", e)
			} else {
				err = fmt.Errorf("kvstore transaction: recovered panic: %v", r)
			}
			_ = t.Rollback()
			return
		}
		if err != nil {
			if txErr := t.Rollback(); txErr != nil {
				s.logger.Errorw("Transaction rollback error", "error", err)
			}
		} else {
			if txErr := t.Commit(); txErr != nil {
				s.logger.Errorw("Transaction commit error", "error", err)
			}
		}
	}()

	return fn(&t)
}

// BeginTx begins a database transaction. If writable is true, then a read/write
// transaction is started, otherwise the transaction will be read-only. Only one
// writable transaction is allowed to be inflight at one time. The caller is
// responsible for closing out the transaction by either calling Commit or Rollback.
func (s *Store) BeginTx(writable bool) (*Transaction, error) {
	var t Transaction
	var err error

	t.writeable = writable
	t.tx, err = s.db.Begin(writable)
	if err != nil {
		return nil, fmt.Errorf("unable to begin transaction: %w", err)
	}

	return &t, nil
}

// Close closes the key/value store.
func (s *Store) Close() error {
	return s.db.Close()
}

// NewStore creates a new Store, backed by a file at filename with mode perm.
func NewStore(logger *logp.Logger, filename string, perm os.FileMode) (*Store, error) {
	var err error

	s := Store{
		logger: logger.Named("kvstore"),
	}
	if s.db, err = bbolt.Open(filename, perm, nil); err != nil {
		return nil, fmt.Errorf("kvstore: unable to open database: %w", err)
	}

	s.logger.Infof("Created new store at %q", filename)

	return &s, nil
}

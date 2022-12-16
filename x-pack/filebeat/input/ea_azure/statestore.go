package ea_azure

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	kvstore "github.com/elastic/beats/v7/filebeat/input/v2/input-kvstore"
	"github.com/elastic/beats/v7/x-pack/libbeat/common/collections"
)

var (
	usersBucket         = []byte("users")
	groupsBucket        = []byte("groups")
	relationshipsBucket = []byte("relationships")
	stateBucket         = []byte("state")

	lastSyncKey         = []byte("last_sync")
	lastUpdateKey       = []byte("last_update")
	usersLinkKey        = []byte("users_link")
	groupsLinkKey       = []byte("groups_link")
	groupMembershipsKey = []byte("group_memberships")
)

type stateStore struct {
	tx *kvstore.Transaction

	lastSync      time.Time
	lastUpdate    time.Time
	usersLink     string
	groupsLink    string
	users         map[uuid.UUID]*User
	groups        map[uuid.UUID]*Group
	relationships *collections.Tree[uuid.UUID]
}

func newStateStore(store *kvstore.Store) (*stateStore, error) {
	tx, err := store.BeginTx(true)
	if err != nil {
		return nil, fmt.Errorf("unable to open state store transaction: %w", err)
	}

	s := stateStore{
		users:         map[uuid.UUID]*User{},
		groups:        map[uuid.UUID]*Group{},
		relationships: collections.NewTree[uuid.UUID](),
		tx:            tx,
	}

	if err = s.tx.Get(stateBucket, lastSyncKey, &s.lastSync); err != nil && !errIsItemNotFound(err) {
		return nil, fmt.Errorf("unable to get last sync time from state: %w", err)
	}
	if err = s.tx.Get(stateBucket, lastUpdateKey, &s.lastUpdate); err != nil && !errIsItemNotFound(err) {
		return nil, fmt.Errorf("unable to get last update time from state: %w", err)
	}
	if err = s.tx.Get(stateBucket, usersLinkKey, &s.usersLink); err != nil && !errIsItemNotFound(err) {
		return nil, fmt.Errorf("unable to get users link from state: %w", err)
	}
	if err = s.tx.Get(stateBucket, groupsLinkKey, &s.groupsLink); err != nil && !errIsItemNotFound(err) {
		return nil, fmt.Errorf("unable to get groups link from state: %w", err)
	}

	if err = s.tx.ForEach(usersBucket, func(key, value []byte) error {
		var u User
		if err = json.Unmarshal(value, &u); err != nil {
			return fmt.Errorf("unable to unmarshal user from state: %w", err)
		}
		s.users[u.ID] = &u

		return nil
	}); err != nil && !errIsItemNotFound(err) {
		return nil, fmt.Errorf("unable to get users from state: %w", err)
	}

	if err = s.tx.ForEach(groupsBucket, func(key, value []byte) error {
		var g Group
		if err = json.Unmarshal(value, &g); err != nil {
			return fmt.Errorf("unable to unmarshal group from state: %w", err)
		}
		s.groups[g.ID] = &g

		return nil
	}); err != nil && !errIsItemNotFound(err) {
		return nil, fmt.Errorf("unable to get users from state: %w", err)
	}

	if err = s.tx.Get(relationshipsBucket, groupMembershipsKey, s.relationships); err != nil && !errIsItemNotFound(err) {
		return nil, fmt.Errorf("unable to get groups relationships from state: %w", err)
	}

	return &s, nil
}

func (s *stateStore) storeUser(u *User) {
	if existing, ok := s.users[u.ID]; ok {
		u.Modified = true
		existing.Merge(u)
	} else {
		u.Added = true
		s.users[u.ID] = u

	}
}

func (s *stateStore) storeGroup(g *Group) {
	s.groups[g.ID] = g
}

func (s *stateStore) close(commit bool) error {
	if !commit {
		return s.tx.Rollback()
	}

	if !s.lastSync.IsZero() {
		if err := s.tx.Set(stateBucket, lastSyncKey, &s.lastSync); err != nil {
			return fmt.Errorf("unable to save last sync time to state: %w", err)
		}
	}
	if !s.lastUpdate.IsZero() {
		if err := s.tx.Set(stateBucket, lastUpdateKey, &s.lastUpdate); err != nil {
			return fmt.Errorf("unable to save last update time to state: %w", err)
		}
	}
	if s.usersLink != "" {
		if err := s.tx.Set(stateBucket, usersLinkKey, &s.usersLink); err != nil {
			return fmt.Errorf("unable to save users link to state: %w", err)
		}
	}
	if s.groupsLink != "" {
		if err := s.tx.Set(stateBucket, groupsLinkKey, &s.groupsLink); err != nil {
			return fmt.Errorf("unable to save groups link to state: %w", err)
		}
	}

	for key, value := range s.users {
		if err := s.tx.Set(usersBucket, key[:], value); err != nil {
			return fmt.Errorf("unable to save user %q to state: %w", key, err)
		}
	}
	for key, value := range s.groups {
		if err := s.tx.Set(groupsBucket, key[:], value); err != nil {
			return fmt.Errorf("unable to save group %q to state: %w", key, err)
		}
	}

	if err := s.tx.Set(relationshipsBucket, groupMembershipsKey, s.relationships); err != nil {
		return fmt.Errorf("unable to save group memberships to state: %w", err)
	}

	return s.tx.Commit()
}

func getLastSync(store *kvstore.Store) (time.Time, error) {
	var t time.Time
	err := store.RunTransaction(false, func(tx *kvstore.Transaction) error {
		return tx.Get(stateBucket, lastSyncKey, &t)
	})

	return t, err
}

func getLastUpdate(store *kvstore.Store) (time.Time, error) {
	var t time.Time
	err := store.RunTransaction(false, func(tx *kvstore.Transaction) error {
		return tx.Get(stateBucket, lastUpdateKey, &t)
	})

	return t, err
}

func errIsItemNotFound(err error) bool {
	return errors.Is(err, kvstore.ErrBucketNotFound) || errors.Is(err, kvstore.ErrKeyNotFound)
}

// Package ea_azure provides an Azure Active Directory input for collecting,
// processing, and submitting user entity objects for Entity Analytics.
package ea_azure

import (
	"context"
	"fmt"

	"time"

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/go-concert/ctxtool"
	"github.com/google/uuid"

	"github.com/elastic/beats/v7/filebeat/input/v2"
	kvstore "github.com/elastic/beats/v7/filebeat/input/v2/input-kvstore"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/feature"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/ea_azure/authenticator/oauth2"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/ea_azure/fetcher"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/ea_azure/fetcher/graph"
	"github.com/elastic/beats/v7/x-pack/libbeat/common/collections"
)

const Name = "ea_azure"

var _ kvstore.Input = &azure{}

// azure implements an input for collecting, processing, and submitting user
// entity objects for Entity Analytics.
type azure struct {
	rawConf *config.C
	conf    conf
	fetcher fetcher.Fetcher
	metrics *inputMetrics
	logger  *logp.Logger
}

// Test checks the configuration and runs additional checks if this input can
// actually collect data for the given configuration. Currently, it is a no-op.
func (a *azure) Test(testCtx v2.TestContext, source kvstore.Source) error {
	//TODO implement me
	return nil
}

// Run starts the data collection.
func (a *azure) Run(inputCtx v2.Context, source kvstore.Source, store *kvstore.Store, client beat.Client) error {
	streamCfg := source.(*stream)
	a.logger = inputCtx.Logger.With("tenant_id", streamCfg.tenantID)
	metricRegistry := monitoring.GetNamespace("dataset").GetRegistry()
	a.metrics = newMetrics(metricRegistry, inputCtx.ID)

	auth, err := oauth2.New(a.rawConf, a.logger)
	if err != nil {
		return fmt.Errorf("unable to create authenticator: %w", err)
	}
	a.fetcher, err = graph.New(a.rawConf, a.logger, auth)
	if err != nil {
		return fmt.Errorf("unable to create fetcher: %w", err)
	}

	// Configure initial timers.
	lastSyncTime, _ := getLastSync(store)
	lastUpdateTime, _ := getLastSync(store)
	syncWaitTime := computeWaitTime(lastSyncTime, a.conf.SyncInterval)
	updateWaitTime := computeWaitTime(lastUpdateTime, a.conf.UpdateInterval)

	// If sync hasn't occurred yet, then queue up update after sync.
	if lastSyncTime.IsZero() {
		updateWaitTime = a.conf.UpdateInterval
	}

	a.logger.Debugf("Initial syncWaitTime: %v updateWaitTime: %v", syncWaitTime, updateWaitTime)

	syncTimer := time.NewTimer(syncWaitTime)
	updateTimer := time.NewTimer(updateWaitTime)

	for {
		select {
		case <-inputCtx.Cancelation.Done():
			if inputCtx.Cancelation.Err() != context.Canceled {
				return inputCtx.Cancelation.Err()
			}
			return nil
		case <-syncTimer.C:
			if err := a.runFullSync(inputCtx, store, client); err != nil {
				a.logger.Errorf("Error running full sync: %v", err)
				a.metrics.fullSyncFailure.Inc()
			} else {
				a.metrics.fullSyncSuccess.Inc()
			}
			a.metrics.fullSyncTotal.Inc()
			updateTimer.Reset(a.conf.SyncInterval)
		case <-updateTimer.C:
			if err := a.runIncrementalUpdate(inputCtx, store, client); err != nil {
				a.logger.Errorf("Error running incremental update: %v", err)
				a.metrics.incrementalUpdateFailure.Inc()
			} else {
				a.metrics.incrementalUpdateSuccess.Inc()
			}
			a.metrics.incrementalUpdateTotal.Inc()
			updateTimer.Reset(a.conf.UpdateInterval)
		}
	}
}

// Name reports the input name.
func (a *azure) Name() string {
	return Name
}

func (a *azure) runFullSync(inputCtx v2.Context, store *kvstore.Store, client beat.Client) (err error) {
	a.logger.Infof("Running full sync...")

	a.logger.Debugf("Opening new transaction...")
	state, err := newStateStore(store)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	a.logger.Debugf("Transaction opened")
	defer func() { // If commit is successful, call to this close will be no-op.
		if err = state.close(false); err != nil {
			a.logger.Errorf("Error rolling back transaction: %v", err)
		}
	}()

	ctx := ctxtool.FromCanceller(inputCtx.Cancelation)
	a.logger.Debugf("Starting fetch...")
	if _, err := a.doFetch(ctx, state); err != nil {
		return err
	}

	if len(state.users) > 0 {
		tracker := kvstore.NewTxTracker(ctx)
		for _, u := range state.users {
			a.publishUser(u, state, inputCtx.ID, client, tracker)
		}

		// TODO: Need to verify no dropped events?
		tracker.Wait()
	}

	state.lastSync = time.Now()
	if err = state.close(true); err != nil {
		return fmt.Errorf("unable to commit state: %w", err)
	}

	return nil
}

func (a *azure) runIncrementalUpdate(inputCtx v2.Context, store *kvstore.Store, client beat.Client) (err error) {
	a.logger.Infof("Running incremental update...")

	state, err := newStateStore(store)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer func() { // If commit is successful, call to this close will be no-op.
		if err = state.close(false); err != nil {
			a.logger.Errorf("Error rolling back transaction: %v", err)
		}
	}()

	ctx := ctxtool.FromCanceller(inputCtx.Cancelation)
	updatedUsers, err := a.doFetch(ctx, state)
	if err != nil {
		return err
	}

	if updatedUsers.Len() > 0 {
		tracker := kvstore.NewTxTracker(ctx)
		updatedUsers.ForEach(func(id uuid.UUID) {
			u, ok := state.users[id]
			if !ok {
				a.logger.Warnf("Unable to lookup user %q", id, u.ID)
				return
			}
			a.publishUser(u, state, inputCtx.ID, client, tracker)
		})

		// TODO: Need to verify no dropped events?
		tracker.Wait()
	}

	state.lastUpdate = time.Now()
	if err = state.close(true); err != nil {
		return fmt.Errorf("unable to commit state: %w", err)
	}

	return nil
}

func (a *azure) doFetch(ctx context.Context, state *stateStore) (*collections.Set[uuid.UUID], error) {
	updatedUsers := collections.NewSet[uuid.UUID]()

	// Get user changes.
	changedUsers, userLink, err := a.fetcher.Users(ctx, state.usersLink)
	if err != nil {
		return nil, err
	}
	a.logger.Debugf("Got %d users from API", len(changedUsers))

	// Get group changes.
	changedGroups, groupLink, err := a.fetcher.Groups(ctx, state.groupsLink)
	if err != nil {
		return nil, err
	}
	a.logger.Debugf("Got %d groups from API", len(changedGroups))

	state.usersLink = userLink
	state.groupsLink = groupLink

	for _, v := range changedUsers {
		updatedUsers.Add(v.ID)
		state.storeUser(v)
	}
	for _, v := range changedGroups {
		state.storeGroup(v)
	}

	// Populate group relationships tree.
	for _, g := range changedGroups {
		state.relationships.AddVertex(g.ID)
		for _, member := range g.Members {
			switch member.Type {
			case fetcher.MemberGroup:
				for _, u := range state.users {
					if u.IsTransitiveMemberOf(member.ID) {
						updatedUsers.Add(u.ID)
					}
				}
				if member.Deleted {
					state.relationships.DeleteEdge(member.ID, g.ID)
				} else {
					state.relationships.AddEdge(member.ID, g.ID)
				}

			case fetcher.MemberUser:
				if u, ok := state.users[member.ID]; ok {
					updatedUsers.Add(u.ID)
					if member.Deleted {
						u.RemoveMemberOf(g.ID)
					} else {
						u.AddMemberOf(g.ID)
					}
				}
			}
		}
	}

	// Expand user group memberships.
	updatedUsers.ForEach(func(userID uuid.UUID) {
		u, ok := state.users[userID]
		if !ok {
			a.logger.Errorf("Unable to find user %q in state", userID)
			return
		}
		if u.Deleted {
			return
		}

		u.TransitiveMemberOf = state.relationships.ExpandFromSet(u.MemberOf)
	})

	return updatedUsers, nil
}

func (a *azure) publishUser(u *fetcher.User, state *stateStore, inputID string, client beat.Client, tracker *kvstore.TxTracker) {
	userDoc := mapstr.M{}

	_, _ = userDoc.Put("azure_ad", u.Fields)
	_, _ = userDoc.Put("ecs.version", "8.5.0")
	_, _ = userDoc.Put("event.kind", "state")
	_, _ = userDoc.Put("event.provider", "Azure AD")
	_, _ = userDoc.Put("event.type", "user")
	_, _ = userDoc.Put("labels.identity_source", inputID)
	_, _ = userDoc.Put("user.id", u.ID.String())

	if u.Deleted {
		_, _ = userDoc.Put("event.action", "user-deleted")
	} else if u.Modified {
		_, _ = userDoc.Put("event.action", "user-modified")
	}

	var groups []fetcher.GroupECS
	u.TransitiveMemberOf.ForEach(func(groupID uuid.UUID) {
		g, ok := state.groups[groupID]
		if !ok {
			a.logger.Warnf("Unable to lookup group %q for user %q", groupID, u.ID)
			return
		}
		groups = append(groups, g.ToECS())
	})
	if len(groups) > 0 {
		_, _ = userDoc.Put("user.group", groups)
	}

	event := beat.Event{
		Timestamp: time.Now(),
		Fields:    userDoc,
		Private:   tracker,
	}
	tracker.Add()

	client.Publish(event)
}

func configure(cfg *config.C) (kvstore.Input, []kvstore.Source, error) {
	c := defaultConf()
	if err := cfg.Unpack(&c); err != nil {
		return nil, nil, fmt.Errorf("unable to unpack %s input config: %w", Name, err)
	}

	var sources []kvstore.Source
	sources = append(sources, &stream{tenantID: c.TenantID})

	return &azure{rawConf: cfg, conf: c}, sources, nil
}

// Plugin describes the type of this input.
func Plugin(logger *logp.Logger) v2.Plugin {
	return v2.Plugin{
		Name:      Name,
		Stability: feature.Experimental,
		Info:      "Azure AD user identities",
		Doc:       "Collect user identities from Azure AD for Entity Analytics",
		Manager: &kvstore.Manager{
			Logger:    logger,
			Type:      Name,
			Configure: configure,
		},
	}
}

// stream represents an identity stream. TODO: Need a better representation?
type stream struct {
	tenantID string
}

func (s *stream) Name() string {
	return s.tenantID
}

func computeWaitTime(last time.Time, duration time.Duration) time.Duration {
	if last.IsZero() {
		return 0
	}
	waitTime := last.Add(duration).Sub(time.Now())
	if waitTime <= 0 {
		return 0
	}

	return waitTime
}

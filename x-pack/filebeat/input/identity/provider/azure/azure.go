package azure

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
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/internal/collections"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/internal/kvstore"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/azure/authenticator"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/azure/authenticator/oauth2"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/azure/fetcher"
	"github.com/elastic/beats/v7/x-pack/filebeat/input/identity/provider/azure/fetcher/graph"
)

const Name = "azure"
const FullName = "identity-" + Name

// azure implements the provider.Provider interface.
var _ provider.Provider = &azure{}

type azure struct {
	*kvstore.Manager

	conf conf

	metrics *inputMetrics
	logger  *logp.Logger
	auth    authenticator.Authenticator
	fetcher fetcher.Fetcher
}

func (p *azure) Name() string {
	return FullName
}

func (p *azure) Test(testCtx v2.TestContext) error {
	p.logger = testCtx.Logger.With("tenant_id", p.conf.TenantID, "provider", Name)
	p.auth.SetLogger(p.logger)

	ctx := ctxtool.FromCanceller(testCtx.Cancelation)
	if _, err := p.auth.Token(ctx); err != nil {
		return fmt.Errorf("%s test failed: %w", Name, err)
	}

	return nil
}

func (p *azure) Run(inputCtx v2.Context, store *kvstore.Store, client beat.Client) error {
	p.logger = inputCtx.Logger.With("tenant_id", p.conf.TenantID, "provider", Name)
	p.auth.SetLogger(p.logger)
	p.fetcher.SetLogger(p.logger)

	metricRegistry := monitoring.GetNamespace("dataset").GetRegistry()
	p.metrics = newMetrics(metricRegistry, inputCtx.ID)

	// Configure initial timers.
	lastSyncTime, _ := getLastSync(store)
	lastUpdateTime, _ := getLastUpdate(store)
	syncWaitTime := computeWaitTime(lastSyncTime, p.conf.SyncInterval)
	updateWaitTime := computeWaitTime(lastUpdateTime, p.conf.UpdateInterval)

	// If sync hasn't occurred yet, then queue up update after sync.
	if lastSyncTime.IsZero() {
		updateWaitTime = p.conf.UpdateInterval
	}

	p.logger.Debugf("Initial syncWaitTime: %v updateWaitTime: %v", syncWaitTime, updateWaitTime)

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
			if err := p.runFullSync(inputCtx, store, client); err != nil {
				p.logger.Errorf("Error running full sync: %v", err)
				p.metrics.fullSyncFailure.Inc()
			} else {
				p.metrics.fullSyncSuccess.Inc()
			}
			p.metrics.fullSyncTotal.Inc()
			updateTimer.Reset(p.conf.SyncInterval)
		case <-updateTimer.C:
			if err := p.runIncrementalUpdate(inputCtx, store, client); err != nil {
				p.logger.Errorf("Error running incremental update: %v", err)
				p.metrics.incrementalUpdateFailure.Inc()
			} else {
				p.metrics.incrementalUpdateSuccess.Inc()
			}
			p.metrics.incrementalUpdateTotal.Inc()
			updateTimer.Reset(p.conf.UpdateInterval)
		}
	}
}

func (p *azure) runFullSync(inputCtx v2.Context, store *kvstore.Store, client beat.Client) (err error) {
	p.logger.Infof("Running full sync...")

	p.logger.Debugf("Opening new transaction...")
	state, err := newStateStore(store)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	p.logger.Debugf("Transaction opened")
	defer func() { // If commit is successful, call to this close will be no-op.
		if err = state.close(false); err != nil {
			p.logger.Errorf("Error rolling back transaction: %v", err)
		}
	}()

	ctx := ctxtool.FromCanceller(inputCtx.Cancelation)
	p.logger.Debugf("Starting fetch...")
	if _, err := p.doFetch(ctx, state); err != nil {
		return err
	}

	if len(state.users) > 0 {
		tracker := kvstore.NewTxTracker(ctx)
		for _, u := range state.users {
			p.publishUser(u, state, inputCtx.ID, client, tracker)
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

func (p *azure) runIncrementalUpdate(inputCtx v2.Context, store *kvstore.Store, client beat.Client) (err error) {
	p.logger.Infof("Running incremental update...")

	state, err := newStateStore(store)
	if err != nil {
		return fmt.Errorf("unable to begin transaction: %w", err)
	}
	defer func() { // If commit is successful, call to this close will be no-op.
		if err = state.close(false); err != nil {
			p.logger.Errorf("Error rolling back transaction: %v", err)
		}
	}()

	ctx := ctxtool.FromCanceller(inputCtx.Cancelation)
	updatedUsers, err := p.doFetch(ctx, state)
	if err != nil {
		return err
	}

	if updatedUsers.Len() > 0 {
		tracker := kvstore.NewTxTracker(ctx)
		updatedUsers.ForEach(func(id uuid.UUID) {
			u, ok := state.users[id]
			if !ok {
				p.logger.Warnf("Unable to lookup user %q", id, u.ID)
				return
			}
			p.publishUser(u, state, inputCtx.ID, client, tracker)
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

func (p *azure) doFetch(ctx context.Context, state *stateStore) (*collections.Set[uuid.UUID], error) {
	updatedUsers := collections.NewSet[uuid.UUID]()

	// Get user changes.
	changedUsers, userLink, err := p.fetcher.Users(ctx, state.usersLink)
	if err != nil {
		return nil, err
	}
	p.logger.Debugf("Got %d users from API", len(changedUsers))

	// Get group changes.
	changedGroups, groupLink, err := p.fetcher.Groups(ctx, state.groupsLink)
	if err != nil {
		return nil, err
	}
	p.logger.Debugf("Got %d groups from API", len(changedGroups))

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
			p.logger.Errorf("Unable to find user %q in state", userID)
			return
		}
		if u.Deleted {
			return
		}

		u.TransitiveMemberOf = state.relationships.ExpandFromSet(u.MemberOf)
	})

	return updatedUsers, nil
}

func (p *azure) publishUser(u *fetcher.User, state *stateStore, inputID string, client beat.Client, tracker *kvstore.TxTracker) {
	userDoc := mapstr.M{}

	_, _ = userDoc.Put("azure_ad", u.Fields)
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
			p.logger.Warnf("Unable to lookup group %q for user %q", groupID, u.ID)
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

func (p *azure) configure(cfg *config.C) (kvstore.Input, error) {
	var err error

	c := defaultConf()
	if err = cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("unable to unpack %s input config: %w", Name, err)
	}

	if p.auth, err = oauth2.New(cfg, p.Manager.Logger); err != nil {
		return nil, fmt.Errorf("unable to create authenticator: %w", err)
	}
	if p.fetcher, err = graph.New(cfg, p.Manager.Logger, p.auth); err != nil {
		return nil, fmt.Errorf("unable to create fetcher: %w", err)
	}

	return p, nil
}

func New(logger *logp.Logger) (provider.Provider, error) {
	p := azure{}
	p.Manager = &kvstore.Manager{
		Logger:    logger,
		Type:      FullName,
		Configure: p.configure,
	}

	return &p, nil
}

func init() {
	if err := provider.Register(Name, New); err != nil {
		panic(err)
	}
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

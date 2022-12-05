// Package ea_azure provides an Azure Active Directory input for collecting,
// processing, and submitting user entity objects for Entity Analytics.
package ea_azure

import (
	"context"
	"fmt"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"time"

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/go-concert/ctxtool"
	"github.com/google/uuid"

	"github.com/elastic/beats/v7/filebeat/input/v2"
	kvstore "github.com/elastic/beats/v7/filebeat/input/v2/input-kvstore"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/feature"
)

const Name = "ea_azure"

var _ kvstore.Input = &azure{}

// azure implements an input for collecting, processing, and submitting user
// entity objects for Entity Analytics.
type azure struct {
	conf    conf
	metrics *inputMetrics
	logger  *logp.Logger

	_authToken   string
	tokenExpires time.Time
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

	// Configure initial timers.
	lastSyncTime, _ := getLastSync(store)
	lastUpdateTime, _ := getLastSync(store)
	syncWaitTime := computeWaitTime(lastSyncTime, a.conf.SyncInterval)
	updateWaitTime := computeWaitTime(lastUpdateTime, a.conf.UpdateInterval)

	// If sync hasn't occurred yet, then queue up update after sync.
	if lastSyncTime.IsZero() {
		updateWaitTime = a.conf.UpdateInterval
	}

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
	if _, err = a.doFetch(ctx, state); err != nil {
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

func (a *azure) publishUser(u *user, state *stateStore, inputID string, client beat.Client, tracker *kvstore.TxTracker) {
	userDoc := mapstr.M{}

	_, _ = userDoc.Put("azure_ad", u.Fields)
	_, _ = userDoc.Put("ecs.version", "8.5.0")
	_, _ = userDoc.Put("event.provider", "Azure AD")
	_, _ = userDoc.Put("event.type", "user")
	_, _ = userDoc.Put("labels.identity_source", inputID)
	if u.Deleted {
		_, _ = userDoc.Put("event.action", "user-deleted")
	}

	var groups []groupECS
	u.TransitiveMemberOf.ForEach(func(groupID uuid.UUID) {
		g, ok := state.groups[groupID]
		if !ok {
			a.logger.Warnf("Unable to lookup group %q for user %q", groupID, u.ID)
			return
		}
		groups = append(groups, g.toECS())
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

// newAzure creates a new azure input instance from the provided configuration.
func newAzure(conf conf) (*azure, error) {
	a := azure{
		conf: conf,
	}

	return &a, nil
}

func configure(cfg *config.C) (kvstore.Input, []kvstore.Source, error) {
	c := defaultConf()
	if err := cfg.Unpack(&c); err != nil {
		return nil, nil, fmt.Errorf("unable to unpack %s input config: %w", Name, err)
	}

	a, err := newAzure(c)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create %s input instance: %w", Name, err)
	}

	var sources []kvstore.Source
	sources = append(sources, &stream{tenantID: c.TenantID})

	return a, sources, nil
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
		return time.Nanosecond
	}
	waitTime := last.Add(duration).Sub(time.Now())
	if waitTime <= 0 {
		return time.Nanosecond
	}

	return waitTime
}

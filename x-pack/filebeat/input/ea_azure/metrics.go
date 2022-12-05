package ea_azure

import "github.com/elastic/elastic-agent-libs/monitoring"

type inputMetrics struct {
	id       string
	registry *monitoring.Registry

	usersAPICallsTotal       *monitoring.Uint
	usersAPICallsSuccess     *monitoring.Uint
	usersAPICallsFailure     *monitoring.Uint
	groupsAPICallsTotal      *monitoring.Uint
	groupsAPICallsSuccess    *monitoring.Uint
	groupsAPICallsFailure    *monitoring.Uint
	fullSyncTotal            *monitoring.Uint
	fullSyncSuccess          *monitoring.Uint
	fullSyncFailure          *monitoring.Uint
	incrementalUpdateTotal   *monitoring.Uint
	incrementalUpdateSuccess *monitoring.Uint
	incrementalUpdateFailure *monitoring.Uint
}

func (m *inputMetrics) Close() {
	m.registry.Remove(m.id)
}

func newMetrics(registry *monitoring.Registry, id string) *inputMetrics {
	reg := registry.NewRegistry(id)

	monitoring.NewString(reg, "input").Set(Name)
	monitoring.NewString(reg, "id").Set(id)

	m := inputMetrics{
		id:                       id,
		registry:                 registry,
		usersAPICallsTotal:       monitoring.NewUint(reg, "api_calls.users.total"),
		usersAPICallsSuccess:     monitoring.NewUint(reg, "api_calls.users.success"),
		usersAPICallsFailure:     monitoring.NewUint(reg, "api_calls.users.failure"),
		groupsAPICallsTotal:      monitoring.NewUint(reg, "api_calls.groups.total"),
		groupsAPICallsSuccess:    monitoring.NewUint(reg, "api_calls.groups.success"),
		groupsAPICallsFailure:    monitoring.NewUint(reg, "api_calls.groups.failure"),
		fullSyncTotal:            monitoring.NewUint(reg, "sync.full.total"),
		fullSyncSuccess:          monitoring.NewUint(reg, "sync.full.success"),
		fullSyncFailure:          monitoring.NewUint(reg, "sync.full.failure"),
		incrementalUpdateTotal:   monitoring.NewUint(reg, "sync.incremental.total"),
		incrementalUpdateSuccess: monitoring.NewUint(reg, "sync.incremental.success"),
		incrementalUpdateFailure: monitoring.NewUint(reg, "sync.incremental.failure"),
	}

	return &m
}

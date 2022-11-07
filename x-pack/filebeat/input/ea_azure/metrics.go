package ea_azure

import "github.com/elastic/elastic-agent-libs/monitoring"

type inputMetrics struct {
	id       string
	registry *monitoring.Registry

	usersTotal          *monitoring.Uint
	groupsTotal         *monitoring.Uint
	usersAPICallsTotal  *monitoring.Uint
	groupsAPICallsTotal *monitoring.Uint
}

func (m *inputMetrics) Close() {
	m.registry.Remove(m.id)
}

func newMetrics(registry *monitoring.Registry, id string) *inputMetrics {
	reg := registry.NewRegistry(id)

	monitoring.NewString(reg, "input").Set(Name)
	monitoring.NewString(reg, "id").Set(id)

	m := inputMetrics{
		id:                  id,
		registry:            registry,
		usersTotal:          monitoring.NewUint(reg, "users_total"),
		groupsTotal:         monitoring.NewUint(reg, "groups_total"),
		usersAPICallsTotal:  monitoring.NewUint(reg, "users_api_calls_total"),
		groupsAPICallsTotal: monitoring.NewUint(reg, "groups_api_calls_total"),
	}

	return &m
}

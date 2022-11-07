package ea_azure

import "github.com/google/uuid"

type group struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Deleted bool      `json:"deleted,omitempty"`
}

type groupAPI struct {
	ID           uuid.UUID `json:"id"`
	DisplayName  string    `json:"displayName"`
	MembersDelta []object  `json:"members@delta,omitempty"`
	Removed      *removed  `json:"@removed,omitempty"`
}

func (g *groupAPI) Deleted() bool {
	return g.Removed != nil
}

type object struct {
	ID      uuid.UUID `json:"id"`
	Type    string    `json:"@odata.type"`
	Removed *removed  `json:"@removed,omitempty"`
}

func (o *object) Deleted() bool {
	return o.Removed != nil
}

type removed struct {
	Reason string `json:"reason"`
}

type groupECS struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (g *group) toECS() groupECS {
	return groupECS{
		ID:   g.ID.String(),
		Name: g.Name,
	}
}

func newGroupFromAPI(g *groupAPI) *group {
	return &group{
		ID:      g.ID,
		Name:    g.DisplayName,
		Deleted: g.Deleted(),
	}
}

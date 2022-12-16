package ea_azure

import "github.com/google/uuid"

type MemberType int

const (
	MemberUser MemberType = iota
	MemberGroup
)

type Group struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Deleted bool      `json:"deleted,omitempty"`
	Members []Member  `json:"-"`
}

type Member struct {
	ID      uuid.UUID
	Type    MemberType
	Deleted bool
}

type groupECS struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (g *Group) toECS() groupECS {
	return groupECS{
		ID:   g.ID.String(),
		Name: g.Name,
	}
}

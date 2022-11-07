package ea_azure

import (
	"errors"
	"fmt"
	"time"

	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/google/uuid"

	"github.com/elastic/beats/v7/x-pack/libbeat/common/collections"
)

var defaultUserMappings = map[string]string{
	"id":                "user.id",
	"userPrincipalName": "user.name,append",
	"mail":              "user.email",
	"displayName":       "user.full_name",
	"givenName":         "user.first_name",
	"surname":           "user.last_name",
	"jobTitle":          "user.job_title",
	"officeLocation":    "user.work.location",
	"mobilePhone":       "user.phone,append",
	"businessPhones":    "user.phone,append",
}

type userAPI mapstr.M

type user struct {
	ID                 uuid.UUID                   `json:"id"`
	Fields             mapstr.M                    `json:"fields"`
	MemberOf           *collections.Set[uuid.UUID] `json:"memberOf"`
	TransitiveMemberOf *collections.Set[uuid.UUID] `json:"transitiveMemberOf"`
	Deleted            bool                        `json:"deleted"`
	LastSent           time.Time                   `json:"lastSent"`
}

func (u *user) merge(other *user) {
	if u.ID != other.ID {
		return
	}
	for k, v := range other.Fields {
		u.Fields[k] = v
	}
	other.MemberOf.ForEach(func(elem uuid.UUID) {
		u.addMemberOf(elem)
	})
	other.TransitiveMemberOf.ForEach(func(elem uuid.UUID) {
		u.addTransitiveMemberOf(elem)
	})
	u.Deleted = other.Deleted
}

func (u *user) isDirectMemberOf(value uuid.UUID) bool {
	if u.MemberOf != nil {
		return u.MemberOf.Has(value)
	}

	return false
}

func (u *user) addMemberOf(value uuid.UUID) {
	if u.MemberOf == nil {
		u.MemberOf = collections.NewSet[uuid.UUID](value)
	} else {
		u.MemberOf.Add(value)
	}
}

func (u *user) removeMemberOf(value uuid.UUID) {
	if u.MemberOf != nil {
		u.MemberOf.Remove(value)
	}
}

func (u *user) isTransitiveMemberOf(value uuid.UUID) bool {
	if u.TransitiveMemberOf != nil {
		return u.TransitiveMemberOf.Has(value)
	}

	return false
}

func (u *user) addTransitiveMemberOf(value uuid.UUID) {
	if u.TransitiveMemberOf == nil {
		u.TransitiveMemberOf = collections.NewSet[uuid.UUID](value)
	} else {
		u.TransitiveMemberOf.Add(value)
	}
}

func (u *user) removeTransitiveMemberOf(value uuid.UUID) {
	if u.TransitiveMemberOf != nil {
		u.TransitiveMemberOf.Remove(value)
	}
}

func newUserFromAPI(u userAPI) (*user, error) {
	var newUser user
	var err error

	newUser.Fields = mapstr.M(u)

	if idRaw, ok := newUser.Fields["id"]; ok {
		idStr, _ := idRaw.(string)
		if newUser.ID, err = uuid.Parse(idStr); err != nil {
			return nil, fmt.Errorf("unable to unmarshal user, invalid ID: %w", err)
		}
		delete(newUser.Fields, "id")
	} else {
		return nil, errors.New("user missing required id field")
	}

	if _, ok := newUser.Fields["@removed"]; ok {
		newUser.Deleted = true
		delete(newUser.Fields, "@removed")
	}

	return &newUser, nil
}

package ea_azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/elastic/beats/v7/x-pack/libbeat/common/collections"
)

const (
	apiGroupType = "#microsoft.graph.group"
	apiUserType  = "#microsoft.graph.user"

	groupsAPIEndpoint = "https://graph.microsoft.com/v1.0/groups/delta?$select=displayName,members"
	usersAPIEndpoint  = "https://graph.microsoft.com/v1.0/users/delta"
)

func (a *azure) doFetch(ctx context.Context, state *stateStore) (*collections.Set[uuid.UUID], error) {
	updatedUsers := collections.NewSet[uuid.UUID]()

	c, err := a.conf.Transport.Client()
	if err != nil {
		return nil, fmt.Errorf("unable to create HTTP client: %w", err)
	}

	// Get user changes.
	changedUsers, userLink, err := a.getUsers(ctx, c, state)
	if err != nil {
		return nil, err
	}
	a.logger.Debugf("Got %d users from API", len(changedUsers))
	// Get group changes.
	changedGroups, groupLink, err := a.getGroups(ctx, c, state)
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
		state.storeGroup(newGroupFromAPI(v))
	}

	// Populate group relationships tree.
	for _, g := range changedGroups {
		state.relationships.AddVertex(g.ID)
		for _, member := range g.MembersDelta {
			switch member.Type {
			case apiGroupType:
				for _, u := range state.users {
					if u.isTransitiveMemberOf(member.ID) {
						updatedUsers.Add(u.ID)
					}
				}
				if member.Deleted() {
					state.relationships.DeleteEdge(member.ID, g.ID)
				} else {
					state.relationships.AddEdge(member.ID, g.ID)
				}

			case apiUserType:
				if u, ok := state.users[member.ID]; ok {
					updatedUsers.Add(u.ID)
					if member.Deleted() {
						u.removeMemberOf(g.ID)
					} else {
						u.addMemberOf(g.ID)
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

func (a *azure) getUsers(ctx context.Context, c *http.Client, state *stateStore) ([]*user, string, error) {
	type apiResponse struct {
		NextLink  string    `json:"@odata.nextLink"`
		DeltaLink string    `json:"@odata.deltaLink"`
		Users     []userAPI `json:"value"`
	}

	fetchURL := usersAPIEndpoint
	if state.usersLink != "" {
		fetchURL = state.usersLink
	}

	var users []*user
	for {
		var response apiResponse

		body, err := a.doRequest(ctx, c, http.MethodGet, fetchURL, nil)
		if err != nil {
			return nil, "", fmt.Errorf("unable to fetch users: %v", err)
		}

		dec := json.NewDecoder(body)
		if err = dec.Decode(&response); err != nil {
			_ = body.Close()
			return nil, "", fmt.Errorf("unable to decode users response: %v", err)
		}
		_ = body.Close()

		for _, v := range response.Users {
			user, err := newUserFromAPI(v)
			if err != nil {
				a.logger.Errorf("Unable to parse user from API: %v", err)
				continue
			}
			a.logger.Debugf("Got user %q from API", user.ID)
			users = append(users, user)
		}

		if response.DeltaLink != "" {
			return users, response.DeltaLink, nil
		}
		if response.NextLink == fetchURL {
			return nil, "", fmt.Errorf("error during fetch users, encountered nextLink fetch infinite loop")
		}
		if response.NextLink != "" {
			fetchURL = response.NextLink
		} else {
			return nil, "", fmt.Errorf("error during fetch users, encountered response without nextLink or deltaLink")
		}
	}
}

func (a *azure) getGroups(ctx context.Context, c *http.Client, state *stateStore) ([]*groupAPI, string, error) {
	type apiResponse struct {
		NextLink  string      `json:"@odata.nextLink"`
		DeltaLink string      `json:"@odata.deltaLink"`
		Groups    []*groupAPI `json:"value"`
	}

	fetchURL := groupsAPIEndpoint
	if state.groupsLink != "" {
		fetchURL = state.groupsLink
	}

	var groups []*groupAPI
	for {
		var response apiResponse

		body, err := a.doRequest(ctx, c, http.MethodGet, fetchURL, nil)
		if err != nil {
			return nil, "", fmt.Errorf("unable to fetch groups: %v", err)
		}

		dec := json.NewDecoder(body)
		if err = dec.Decode(&response); err != nil {
			_ = body.Close()
			return nil, "", fmt.Errorf("unable to decode groups response: %v", err)
		}
		_ = body.Close()

		for _, v := range response.Groups {
			a.logger.Debugf("Got group %q from API", v.ID)
			groups = append(groups, v)
		}

		if response.DeltaLink != "" {
			return groups, response.DeltaLink, nil
		}
		if response.NextLink == fetchURL {
			return nil, "", fmt.Errorf("error during fetch groups, encountered nextLink fetch infinite loop")
		}
		if response.NextLink != "" {
			fetchURL = response.NextLink
		} else {
			return nil, "", fmt.Errorf("error during fetch groups, encountered response without nextLink or deltaLink")
		}
	}
}

func (a *azure) doRequest(ctx context.Context, c *http.Client, method, url string, body io.Reader) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}
	bearer, err := a.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get bearer token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)

	res, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		bodyData, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status code: %d body: %s", res.StatusCode, string(bodyData))
	}

	return res.Body, nil
}

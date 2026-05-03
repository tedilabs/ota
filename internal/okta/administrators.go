package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tedilabs/ota/internal/domain"
)

// AdministratorsAdapter implements domain.AdministratorsPort.
//
// Okta exposes admin role assignments via two endpoints:
//
//   - /api/v1/iam/assignees/users — flat list of user IDs / logins
//     who carry at least one admin role.
//   - /api/v1/users/{id}/roles    — per-user list of resolved roles.
//
// MVP fans out the second per-user fetch so we can produce the
// (user, role) flat-row shape the TUI renders. For orgs with > 100
// admins this is N+1; revisit when Okta ships a server-side join.
type AdministratorsAdapter struct{ client *Client }

func (a *AdministratorsAdapter) List(ctx context.Context) ([]domain.Administrator, error) {
	users, err := a.listAssigneeUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Administrator, 0, len(users))
	for _, u := range users {
		roles, err := a.listUserRoles(ctx, u.ID)
		if err != nil {
			// Skip the row but don't fail the whole fetch — operator
			// would rather see N-1 admins than an empty error panel.
			continue
		}
		for _, r := range roles {
			out = append(out, domain.Administrator{
				UserID:    u.ID,
				Login:     u.Login,
				FirstName: u.FirstName,
				LastName:  u.LastName,
				RoleID:    r.ID,
				RoleType:  r.Type,
				RoleLabel: r.Label,
				Status:    r.Status,
			})
		}
	}
	return out, nil
}

type wireAssigneeUser struct {
	ID        string `json:"id"`
	Login     string `json:"-"`
	FirstName string `json:"-"`
	LastName  string `json:"-"`
	Profile   struct {
		Login     string `json:"login"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	} `json:"profile"`
}

type wireAdminRole struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Label  string `json:"label"`
	Status string `json:"status"`
}

// listAssigneeUsers reads /api/v1/iam/assignees/users and flattens
// the response so the caller doesn't have to dig through nested
// profile fields.
func (a *AdministratorsAdapter) listAssigneeUsers(ctx context.Context) ([]wireAssigneeUser, error) {
	u := a.client.buildURL("/api/v1/iam/assignees/users")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)
	var users []wireAssigneeUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("okta: assignees decode: %w", err)
	}
	for i := range users {
		users[i].Login = users[i].Profile.Login
		users[i].FirstName = users[i].Profile.FirstName
		users[i].LastName = users[i].Profile.LastName
	}
	return users, nil
}

func (a *AdministratorsAdapter) listUserRoles(ctx context.Context, userID string) ([]wireAdminRole, error) {
	u := a.client.buildURL("/api/v1/users/" + url.PathEscape(userID) + "/roles")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)
	var roles []wireAdminRole
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, fmt.Errorf("okta: user roles decode: %w", err)
	}
	return roles, nil
}

package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tedilabs/ota/internal/domain"
)

// UpdateProfile issues PUT /api/v1/groups/{groupID} with the full
// replacement profile (Okta's group-update semantics are
// strict-replace — every field must be present in the body).
// Returns the server-echoed Group so the screen can patch its cache.
//
// Callers must guard on Type == OKTA_GROUP before invoking. The
// adapter does not re-check; APP_GROUP / BUILT_IN return 403 from
// the API (errormap maps it to ErrForbidden).
func (a *GroupsAdapter) UpdateProfile(ctx context.Context, groupID string, profile domain.GroupProfileUpdate) (domain.Group, error) {
	body, err := json.Marshal(wireGroupUpdateBody{Profile: wireGroupProfile{
		Name:        profile.Name,
		Description: profile.Description,
	}})
	if err != nil {
		return domain.Group{}, fmt.Errorf("okta: marshal group update body: %w", err)
	}

	u := a.client.buildURL("/api/v1/groups/" + url.PathEscape(groupID))
	resp, err := a.client.doPut(ctx, u, body)
	if err != nil {
		return domain.Group{}, err
	}
	defer drainAndClose(resp)

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return domain.Group{}, fmt.Errorf("okta: read group update response: %w", err)
	}
	var wg wireGroup
	if err := json.Unmarshal(buf.Bytes(), &wg); err != nil {
		return domain.Group{}, fmt.Errorf("okta: decode group update response: %w", err)
	}
	return mapGroup(&wg), nil
}

// wireGroupUpdateBody is the request envelope for
// PUT /api/v1/groups/{id}. Profile is required by Okta even when
// only one field changes — strict-replace semantics.
type wireGroupUpdateBody struct {
	Profile wireGroupProfile `json:"profile"`
}

// wireGroupProfile mirrors the strict shape Okta expects on the wire.
// Pointer / omitempty intentionally avoided — the API treats absent
// fields as null and clears them; the screen always sends the
// current value for unchanged fields.
type wireGroupProfile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

package okta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tedilabs/ota/internal/domain"
)

// UpdateProfile issues POST /api/v1/users/{userID} with a partial-merge
// JSON body (REQ-W01 AC-4.2 / D-T9). PUT (strict-replace) is NEVER
// exposed (D-T9 — depguard guards the SDK PUT helper from importing
// here in v0.2).
//
// Guards (D-T5 / D-W13):
//   - patch.IsEmpty() == true → return domain.ErrEmptyPatch WITHOUT
//     making an HTTP call. The TUI's Save-disabled-at-dirty-zero is
//     the primary guard; this is the defensive backstop.
//
// Body shape (D-T4):
//   - Marshals to `{"profile": {<dirty-fields-only>}}` — nil pointer
//     fields are OMITTED via `json:",omitempty"` on the wire struct.
//
// Response: server echoes the full User. Domain User translated via
// mapUser so the caller can patch its list/detail cache for
// last-write-wins reconciliation (domain §5.2-2).
func (a *UsersAdapter) UpdateProfile(ctx context.Context, userID string, patch domain.UserProfilePatch) (domain.User, error) {
	if patch.IsEmpty() {
		return domain.User{}, domain.ErrEmptyPatch
	}

	body, err := json.Marshal(wireUserUpdateBody{Profile: profileFromPatch(patch)})
	if err != nil {
		return domain.User{}, fmt.Errorf("okta: marshal user update body: %w", err)
	}

	u := a.client.buildURL("/api/v1/users/" + url.PathEscape(userID))
	resp, err := a.client.doPost(ctx, u, body)
	if err != nil {
		return domain.User{}, err
	}
	defer drainAndClose(resp)

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return domain.User{}, fmt.Errorf("okta: read user update response: %w", err)
	}
	var wu wireUser
	if err := json.Unmarshal(buf.Bytes(), &wu); err != nil {
		return domain.User{}, fmt.Errorf("okta: decode user update response: %w", err)
	}
	return mapUser(&wu), nil
}

// wireUserUpdateBody is the request envelope for POST /api/v1/users/{id}.
// Sparse-encodes the dirty fields under `profile` (D-T4).
type wireUserUpdateBody struct {
	Profile wireUserProfilePatch `json:"profile"`
}

// wireUserProfilePatch mirrors the Okta wire schema using *string
// pointers so unedited fields are omitted on the JSON wire — Okta's
// partial-merge semantics treat absent keys as "unchanged" (D-T4 /
// D-W13). Explicit-null-clear is intentionally not modelled here.
type wireUserProfilePatch struct {
	FirstName      *string `json:"firstName,omitempty"`
	LastName       *string `json:"lastName,omitempty"`
	DisplayName    *string `json:"displayName,omitempty"`
	NickName       *string `json:"nickName,omitempty"`
	Email          *string `json:"email,omitempty"`
	Title          *string `json:"title,omitempty"`
	Division       *string `json:"division,omitempty"`
	Department     *string `json:"department,omitempty"`
	EmployeeNumber *string `json:"employeeNumber,omitempty"`
	MobilePhone    *string `json:"mobilePhone,omitempty"`
	SecondEmail    *string `json:"secondEmail,omitempty"`
}

// profileFromPatch maps the domain.UserProfilePatch into the wire
// shape verbatim — pointer identity is preserved so nil fields stay
// omitted by json.Marshal.
func profileFromPatch(p domain.UserProfilePatch) wireUserProfilePatch {
	return wireUserProfilePatch{
		FirstName:      p.FirstName,
		LastName:       p.LastName,
		DisplayName:    p.DisplayName,
		NickName:       p.NickName,
		Email:          p.Email,
		Title:          p.Title,
		Division:       p.Division,
		Department:     p.Department,
		EmployeeNumber: p.EmployeeNumber,
		MobilePhone:    p.MobilePhone,
		SecondEmail:    p.SecondEmail,
	}
}

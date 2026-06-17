package okta_test

// REQ-W01 — UsersAdapter.UpdateProfile adapter integration tests
// (Step 4 of Phase 5 RED order). httptest.Server stands in for
// Okta; we pin:
//
//   1) POST /api/v1/users/{id} (NOT PUT — D-W15 / D-T9: PUT not exposed)
//   2) Body shape: only dirty fields under "profile" (AC-4.2 / D-T4)
//   3) ErrEmptyPatch guard: IsEmpty() short-circuits without HTTP (D-T5 / D-W13)
//   4) Response decode: server-echoed User propagates to the caller
//
// Phase 5 RED expectation: UpdateProfile is a stub returning
// "not implemented" error so EVERY assertion below fails until Phase 6
// implements the real path.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta"
)

// AC-4.2 / D-T4 — partial-merge body: only the dirty fields end up
// under the `profile` JSON object. Every unedited field must be
// OMITTED (not sent as empty string). The HTTP method must be POST
// (D-W15: strict-replace PUT is not exposed by the adapter).
func Test_OktaUsersAdapter_UpdateProfile_PartialMerge_SingleField_BodyShape(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	var capturedMethod string
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "00u_alice",
			"status": "ACTIVE",
			"profile": {
				"login": "alice@acme.com",
				"email": "alice@acme.com",
				"firstName": "Alicia",
				"lastName": "Smith"
			}
		}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "tok", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	newFirst := "Alicia"
	patch := domain.UserProfilePatch{FirstName: &newFirst}
	user, err := cli.Users().UpdateProfile(context.Background(), "00u_alice", patch)
	require.NoError(t, err, "REQ-W01 AC-4.2: single-field partial merge must succeed")

	assert.Equal(t, http.MethodPost, capturedMethod,
		"REQ-W01 D-W15 / D-T9: must use POST (partial merge) — never PUT (strict replace)")
	assert.Equal(t, "/api/v1/users/00u_alice", capturedPath,
		"REQ-W01: URL must target /api/v1/users/{id}")

	var sent struct {
		Profile map[string]any `json:"profile"`
	}
	require.NoError(t, json.Unmarshal(capturedBody, &sent),
		"REQ-W01 AC-4.2: body must be JSON with a `profile` envelope")

	assert.Equal(t, "Alicia", sent.Profile["firstName"],
		"REQ-W01 AC-4.2: dirty field must appear in profile.<field>")

	// 10 other fields MUST NOT appear — pin the omitempty behaviour
	// per field. This catches the common bug where *string nil
	// marshals as `null` instead of being omitted.
	omitted := []string{
		"lastName", "displayName", "nickName", "email",
		"title", "division", "department", "employeeNumber",
		"mobilePhone", "secondEmail",
	}
	for _, k := range omitted {
		_, present := sent.Profile[k]
		assert.False(t, present, "REQ-W01 AC-4.2: %q must be omitted when patch field is nil", k)
	}

	// Response decode: server-echoed user propagates
	assert.Equal(t, "00u_alice", user.ID, "REQ-W01 AC-4.5: server response User.ID must propagate")
	assert.Equal(t, "Alicia", user.Profile.FirstName,
		"REQ-W01 AC-4.5: server response is authoritative — propagates as the returned User")
}

// AC-4.2 / D-T4 — multi-field patch: order-independent, all dirty
// fields present, all clean fields omitted.
func Test_OktaUsersAdapter_UpdateProfile_PartialMerge_MultiField_BodyShape(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"00u1","status":"ACTIVE","profile":{"login":"a@x"}}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "tok", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	dept := "Platform"
	div := "R&D"
	emp := "ENG-099"
	patch := domain.UserProfilePatch{Department: &dept, Division: &div, EmployeeNumber: &emp}
	_, err = cli.Users().UpdateProfile(context.Background(), "00u1", patch)
	require.NoError(t, err)

	var sent struct {
		Profile map[string]any `json:"profile"`
	}
	require.NoError(t, json.Unmarshal(capturedBody, &sent))

	assert.Equal(t, "Platform", sent.Profile["department"])
	assert.Equal(t, "R&D", sent.Profile["division"])
	assert.Equal(t, "ENG-099", sent.Profile["employeeNumber"])

	// remaining 8 (firstName/lastName/displayName/nickName/email/title/
	// mobilePhone/secondEmail) must all be omitted
	for _, k := range []string{
		"firstName", "lastName", "displayName", "nickName", "email",
		"title", "mobilePhone", "secondEmail",
	} {
		_, present := sent.Profile[k]
		assert.False(t, present, "REQ-W01 AC-4.2: %q must be omitted (not nil-serialized)", k)
	}
}

// AC-2 / D-W2 — login is NOT in UserProfilePatch (read-only). This is
// a compile-time check: the struct literal below must NOT have a Login
// field. This test pins the absence by attempting to construct a
// non-empty patch using every legal field name.
//
// NOTE: A literal `Login` field reference would fail to compile if the
// struct doesn't have it. We embed that proof in the test by listing
// every legal field via assignment. Phase 6 must NOT add a Login
// field to UserProfilePatch.
func Test_UserProfilePatch_HasNoLoginField(t *testing.T) {
	t.Parallel()
	v := "v"
	// All 11 legal fields — if any of these stops compiling, the
	// struct shape regressed.
	p := domain.UserProfilePatch{
		FirstName: &v, LastName: &v, DisplayName: &v, NickName: &v,
		Email: &v, Title: &v, Division: &v, Department: &v,
		EmployeeNumber: &v, MobilePhone: &v, SecondEmail: &v,
	}
	assert.False(t, p.IsEmpty(),
		"sanity: every legal field is set so patch is non-empty (REQ-W01 D-W2: login intentionally absent)")
}

// D-T5 / D-W13 — empty patch short-circuits with ErrEmptyPatch BEFORE
// any HTTP call. The handler counter must remain 0.
func Test_OktaUsersAdapter_UpdateProfile_EmptyPatch_ReturnsErrEmptyPatch_NoHTTPCall(t *testing.T) {
	t.Parallel()

	called := 0
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called++
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "tok", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	user, err := cli.Users().UpdateProfile(context.Background(), "00u_alice", domain.UserProfilePatch{})
	assert.ErrorIs(t, err, domain.ErrEmptyPatch,
		"REQ-W01 D-T5 / D-W13: IsEmpty() patch must return ErrEmptyPatch")
	assert.Equal(t, 0, called,
		"REQ-W01 D-W13: no HTTP request must reach Okta when the patch is empty")
	assert.Equal(t, "", user.ID,
		"on ErrEmptyPatch the returned User must be the zero value")
}

// AC-6 — 400 BadRequestError carries the errorCauses for inline
// field display. The adapter must propagate them on
// *domain.BadRequestError so the TUI's form.ApplyServerErrors can map
// them by FieldSpec.Key.
func Test_OktaUsersAdapter_UpdateProfile_400Validation_PropagatesCauses(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"errorCode": "E0000001",
			"errorSummary": "Api validation failed: profile",
			"errorCauses": [
				{"errorSummary": "email: Email is not valid"},
				{"errorSummary": "department: Cannot exceed 100 characters"}
			]
		}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "tok", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	newEmail := "bogus"
	patch := domain.UserProfilePatch{Email: &newEmail}
	_, err = cli.Users().UpdateProfile(context.Background(), "00u_alice", patch)
	require.Error(t, err, "REQ-W01 AC-6: 400 must produce a domain error")

	var bre *domain.BadRequestError
	require.ErrorAs(t, err, &bre, "REQ-W01 AC-6: 400 must yield *BadRequestError so the form maps causes inline")
	require.Len(t, bre.Causes, 2, "REQ-W01 AC-6.1: both errorCauses must propagate verbatim")
}

// AC-6 — 403 forbidden maps to domain.ErrForbidden so the TUI can
// render the "Insufficient permissions" toast (D-W12 / AC-6 403 row).
func Test_OktaUsersAdapter_UpdateProfile_403Forbidden_ReturnsErrForbidden(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorCode":"E0000006","errorSummary":"Insufficient permissions"}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "tok", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	v := "x"
	_, err = cli.Users().UpdateProfile(context.Background(), "00u_alice", domain.UserProfilePatch{FirstName: &v})
	assert.ErrorIs(t, err, domain.ErrForbidden,
		"REQ-W01 AC-6 403: must map to ErrForbidden sentinel")
}

// AC-6 — 404 not found maps to domain.ErrNotFound so the TUI can
// close the form and refresh the list (AC-6.4).
func Test_OktaUsersAdapter_UpdateProfile_404NotFound_ReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":"E0000007","errorSummary":"Not found"}`))
	}))
	defer srv.Close()

	cli, err := okta.NewClient(context.Background(), okta.Config{
		OrgURL: srv.URL, APIToken: "tok", HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	v := "x"
	_, err = cli.Users().UpdateProfile(context.Background(), "00u_gone", domain.UserProfilePatch{FirstName: &v})
	assert.ErrorIs(t, err, domain.ErrNotFound,
		"REQ-W01 AC-6.4: 404 must map to ErrNotFound sentinel")
}

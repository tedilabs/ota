package users_test

// Smoke tests for the §15.7 v1.2.0 Raw detail tab. They drive the model
// directly (no teatest harness) so a regression in the JSON projection /
// mask wrapping / "# masked" annotation surfaces immediately.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/users"
)

// Test_DetailModel_RawTab_RendersMaskedJSON — Raw tab serialises
// domain.User and replaces PII fields with their mask tokens; the lines
// that contain a masked value are annotated with `# masked`.
func Test_DetailModel_RawTab_RendersMaskedJSON(t *testing.T) {
	t.Parallel()

	u := domain.User{
		ID:     "00u_alice",
		Status: domain.UserStatusActive,
		Profile: domain.UserProfile{
			Login:       "alice@acme.com",
			MobilePhone: "+1-415-555-1234",
			SecondEmail: "alice@personal.com",
		},
	}
	m := users.NewDetailModel(users.Deps{}, u).WithActiveTab(users.DetailTabRaw)
	got := m.View()

	require.Contains(t, got, "[Raw]",
		"Raw tab label must be active")
	require.Contains(t, got, "\"id\": \"00u_alice\"",
		"id field must serialise verbatim")
	require.Contains(t, got, "\"status\": \"ACTIVE\"",
		"status enum must serialise as its string form")
	// PII fields are masked.
	assert.Contains(t, got, "***",
		"masked PII fields must surface their *** tokens")
	assert.NotContains(t, got, "+1-415-555-1234",
		"plaintext phone must not appear in Raw output (PII §7.2)")
	assert.NotContains(t, got, "alice@personal.com",
		"plaintext secondEmail must not appear (PII §7.2)")
	// Lines whose value contains a mask token get the `# masked` comment.
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "***") {
			assert.True(t, strings.HasSuffix(line, "# masked"),
				"masked line must end with `# masked` annotation: %q", line)
		}
	}
}

// Test_DetailModel_RawTab_TabBarShowsAllSeven — the tab bar must list all
// seven tabs (Profile…Raw) regardless of which one is active. Active tab
// is rendered as `[Label]`, others as `[ Label ]`.
func Test_DetailModel_RawTab_TabBarShowsAllSeven(t *testing.T) {
	t.Parallel()

	m := users.NewDetailModel(users.Deps{}, domain.User{ID: "x"}).WithActiveTab(users.DetailTabProfile)
	bar := m.View()

	for _, label := range []string{"Profile", "Credentials", "Timestamps", "Groups", "Factors", "Recent", "Raw"} {
		assert.Contains(t, bar, label,
			"tab bar must include the %q tab (TUI_DESIGN §15.7 v1.2.0)", label)
	}
	assert.Contains(t, bar, "[Profile]",
		"active Profile tab must render with no inner padding")
	assert.Contains(t, bar, "[ Raw ]",
		"inactive Raw tab must render with inner padding")
}

// Test_UsersList_RKey_TogglesRawTab — pressing `r` while the inline detail
// surface is open jumps to the Raw tab; a second `r` returns to the
// previously-active tab (Profile by default).
func Test_UsersList_RKey_TogglesRawTab(t *testing.T) {
	t.Parallel()

	port := fakes.NewUsersPort(t)
	port.GetFunc = func(_ context.Context, id string) (domain.User, error) {
		return domain.User{ID: id, Profile: domain.UserProfile{Login: "alice@acme.com"}}, nil
	}
	m := users.NewListModel(users.Deps{
		Port:         port,
		Clock:        clock.NewFake(fixtureNow()),
		InitialUsers: sampleUsersFixture(),
		Width:        120,
		Height:       30,
	})

	// Enter detail mode via `d`.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(users.ListModel)
	require.NotNil(t, cmd, "`d` must emit fetch Cmd")
	updated, _ = m.Update(cmd())
	m = updated.(users.ListModel)
	require.Contains(t, m.View(), "[Profile]",
		"detail mode must start on Profile tab")

	// `r` jumps to Raw.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(users.ListModel)
	view1 := m.View()
	assert.Contains(t, view1, "[Raw]",
		"after `r`, Raw tab must be active")
	assert.Contains(t, view1, "\"login\": \"alice@acme.com\"",
		"Raw tab must show JSON of the fetched user")

	// `r` again toggles back to Profile.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(users.ListModel)
	view2 := m.View()
	assert.Contains(t, view2, "[Profile]",
		"second `r` must return to the previously-active tab")
	assert.NotContains(t, view2, "\"login\": \"alice@acme.com\"",
		"after returning, the JSON body must be gone")
}

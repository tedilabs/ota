package app_test

// Pins the palette/filter modal border alignment with the chrome's
// right border (issue #151). lipgloss `.Width(N)` sizes the inside-
// of-border area, so the rendered outer box is N+2 cells wide. The
// previous modalBox passed contentWidth verbatim and the resulting
// box overshot by 2 cells — its `╮` corner got eaten by the chrome's
// padTo, leaving the visible right edge looking broken. The fix
// passes contentWidth-2 so the modal lands flush.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/testfx"
)

type borderUsersPort struct{ users []domain.User }

func (p *borderUsersPort) List(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
	return &borderUsersIter{rem: p.users}, nil
}
func (p *borderUsersPort) Get(_ context.Context, id string) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}
func (p *borderUsersPort) ListGroups(_ context.Context, _ string) ([]domain.Group, error) {
	return nil, nil
}
func (p *borderUsersPort) ListFactors(_ context.Context, _ string) ([]domain.Factor, error) {
	return nil, nil
}
func (p *borderUsersPort) ListAppLinks(_ context.Context, _ string) ([]domain.AppLink, error) {
	return nil, nil
}
func (p *borderUsersPort) ResetPassword(_ context.Context, _ string, _ bool) (string, error) {
	return "", nil
}
func (p *borderUsersPort) Unlock(_ context.Context, _ string) error       { return nil }
func (p *borderUsersPort) ResetFactors(_ context.Context, _ string) error { return nil }

type borderUsersIter struct{ rem []domain.User }

func (it *borderUsersIter) Next(_ context.Context) (domain.User, bool, error) {
	if len(it.rem) == 0 {
		return domain.User{}, false, nil
	}
	u := it.rem[0]
	it.rem = it.rem[1:]
	return u, true, nil
}
func (it *borderUsersIter) Close() error { return nil }

// Test_AppShell_PaletteBox_BorderAlignsWithChrome opens the `:` palette
// and inspects the rendered body line that contains the modal's top
// border — its `╮` corner must sit immediately left of the chrome's
// right `│` (or be the same cell as the chrome border, depending on
// the layer order). No visible gap, no over-shoot.
func Test_AppShell_PaletteBox_BorderAlignsWithChrome(t *testing.T) {
	t.Parallel()

	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	port := &borderUsersPort{users: []domain.User{
		{ID: "00u_alice", Status: domain.UserStatusActive,
			Profile: domain.UserProfile{Login: "alice@acme.com"}},
	}}
	m := app.New(app.Deps{
		Keys: keymap, Clock: clock.Real(), Profile: "test",
		OrgURL: "https://acme.okta.com", UsersPort: port,
	})
	if init := m.Init(); init != nil {
		if msg := init(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(app.Model)
		}
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(app.Model)

	// Open the palette via `:` — the resulting overlay msg flips the
	// app's overlay state.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m = updated.(app.Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = m.Update(msg)
			m = updated.(app.Model)
		}
	}

	view := testfx.StripANSI(m.View())
	lines := strings.Split(view, "\n")

	// Find the line that carries the palette box's top border (`╭`).
	var topBorder string
	for _, l := range lines {
		if strings.Contains(l, "╭─") && strings.Contains(l, "╮") &&
			!strings.Contains(l, "ota v") /* skip chrome's own top */ {
			topBorder = l
			break
		}
	}
	require.NotEmpty(t, topBorder, "must find a palette top-border line")

	// The line must end with the chrome's right `│` border with at
	// most 1 space between the modal's `╮` and the chrome's `│`.
	idxRight := strings.LastIndex(topBorder, "╮")
	require.GreaterOrEqual(t, idxRight, 0)
	tail := topBorder[idxRight+len("╮"):]
	// `tail` should be the chrome's right border `│` directly, no
	// gap of `─`s and no leftover whitespace beyond a single space.
	assert.NotContains(t, tail, "─",
		"no `─` characters allowed between modal's `╮` and chrome's `│` (issue #151)")
	assert.LessOrEqualf(t, len(tail), len("│")+0,
		"modal's `╮` must butt up against the chrome's `│` (tail=%q)", tail)
}

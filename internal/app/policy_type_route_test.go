package app_test

// Pins the per-policy-type palette routes (issue #165). `:okta-sign-on`,
// `:password-policy`, etc. must rebuild the Policies wrapper scoped
// directly to the requested type — no picker pass-through.

import (
	"context"
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

type ptPoliciesPort struct {
	listFunc func(context.Context, domain.PoliciesQuery) (domain.Iterator[domain.Policy], error)
}

func (p *ptPoliciesPort) List(ctx context.Context, q domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
	return p.listFunc(ctx, q)
}
func (p *ptPoliciesPort) Get(_ context.Context, _ string) (domain.Policy, error) {
	return domain.Policy{}, domain.ErrNotFound
}
func (p *ptPoliciesPort) Rules(_ context.Context, _ string) ([]domain.PolicyRule, error) {
	return nil, nil
}
func (p *ptPoliciesPort) UpdatePolicy(_ context.Context, _ string, _ domain.PolicyUpdate) (domain.Policy, error) {
	return domain.Policy{}, nil
}

type ptPolicyIter struct{ rem []domain.Policy }

func (it *ptPolicyIter) Next(_ context.Context) (domain.Policy, bool, error) {
	if len(it.rem) == 0 {
		return domain.Policy{}, false, nil
	}
	p := it.rem[0]
	it.rem = it.rem[1:]
	return p, true, nil
}
func (it *ptPolicyIter) Close() error { return nil }

func feedAppKey(t *testing.T, m app.Model, key tea.KeyMsg) (app.Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(key)
	out, ok := updated.(app.Model)
	require.True(t, ok)
	return out, cmd
}

// Test_AppShell_PaletteRoutes_PerPolicyType verifies typing
// `:okta-sign-on<Enter>` jumps straight to ScreenPolicies with a
// list scoped to OKTA_SIGN_ON — the picker doesn't show.
func Test_AppShell_PaletteRoutes_PerPolicyType(t *testing.T) {
	t.Parallel()

	port := &ptPoliciesPort{
		listFunc: func(_ context.Context, q domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
			assert.Equal(t, domain.PolicyTypeOktaSignOn, q.Type,
				"`:okta-sign-on` must scope the list query to OKTA_SIGN_ON")
			return &ptPolicyIter{rem: []domain.Policy{
				{ID: "00p_a", Name: "Default Sign-On", Type: domain.PolicyTypeOktaSignOn,
					Status: domain.PolicyStatusActive, Priority: 1, System: true},
			}}, nil
		},
	}
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys: keymap, Clock: clock.Real(), Profile: "test",
		OrgURL:        "https://acme.okta.com",
		PoliciesPort:  port,
		InitialScreen: app.ScreenUsers,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(app.Model)

	// `:` opens the palette overlay.
	m, cmd := feedAppKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(app.Model)
		}
	}
	for _, r := range "okta-sign-on" {
		m, _ = feedAppKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Enter resolves the palette → produces an OpenPolicyTypeMsg via
	// openPolicyTypeCmd which the App Shell handles by replacing
	// the Policies wrapper.
	m, cmd = feedAppKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter must produce the route Cmd")
	if msg := cmd(); msg != nil {
		updated, c := m.Update(msg)
		m = updated.(app.Model)
		// The wrapper's Init Cmd then loads the typed policies.
		if c != nil {
			if next := c(); next != nil {
				updated, _ := m.Update(next)
				m = updated.(app.Model)
			}
		}
	}

	view := testfx.StripANSI(m.View())
	// Type-select picker must NOT render — picker shows
	// "Select Policy Type" header.
	assert.NotContains(t, view, "Select Policy Type:",
		"picker must be skipped when palette specifies a type")
	// The seeded policy must surface in the typed list.
	assert.Contains(t, view, "Default Sign-On",
		"policies list scoped to OKTA_SIGN_ON must surface seeded entries")
}

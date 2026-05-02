package app_test

// End-to-end regression for the Apps screen palette routing
// (issue #166 follow-up — user reported "app 리소스 뷰가 하나도
// 동작을 안해"). Pins both the picker entry (`:app<Enter>`) and the
// direct typed routes (`:saml-app<Enter>`).

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

type appRouteAppsPort struct {
	apps     []domain.App
	lastType domain.AppType
}

func (p *appRouteAppsPort) List(_ context.Context, q domain.AppsQuery) (domain.Iterator[domain.App], error) {
	p.lastType = q.Type
	out := make([]domain.App, 0, len(p.apps))
	for _, a := range p.apps {
		if q.Type == "" || a.Type == q.Type {
			out = append(out, a)
		}
	}
	return &appRouteIter{rem: out}, nil
}
func (p *appRouteAppsPort) Get(_ context.Context, _ string) (domain.App, error) {
	return domain.App{}, domain.ErrNotFound
}

type appRouteIter struct{ rem []domain.App }

func (it *appRouteIter) Next(_ context.Context) (domain.App, bool, error) {
	if len(it.rem) == 0 {
		return domain.App{}, false, nil
	}
	a := it.rem[0]
	it.rem = it.rem[1:]
	return a, true, nil
}
func (it *appRouteIter) Close() error { return nil }

func appsFixtureForRoute() []domain.App {
	return []domain.App{
		{ID: "0oa1", Name: "salesforce", Label: "Salesforce",
			Status: domain.AppStatusActive, SignOnMode: "SAML_2_0", Type: domain.AppTypeSAML},
		{ID: "0oa2", Name: "okta_org2org", Label: "Org2Org",
			Status: domain.AppStatusActive, SignOnMode: "OPENID_CONNECT", Type: domain.AppTypeOIDC},
	}
}

func bootAppsApp(t *testing.T, port domain.AppsPort) app.Model {
	t.Helper()
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	m := app.New(app.Deps{
		Keys:          keymap,
		Clock:         clock.Real(),
		Profile:       "test",
		OrgURL:        "https://acme.okta.com",
		AppsPort:      port,
		InitialScreen: app.ScreenUsers,
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return updated.(app.Model)
}

func paletteRun(t *testing.T, m app.Model, command string) app.Model {
	t.Helper()
	step := func(mdl app.Model, msg tea.Msg) app.Model {
		updated, cmd := mdl.Update(msg)
		out := updated.(app.Model)
		if cmd != nil {
			if next := cmd(); next != nil {
				updated, c2 := out.Update(next)
				out = updated.(app.Model)
				if c2 != nil {
					if then := c2(); then != nil {
						updated, _ = out.Update(then)
						out = updated.(app.Model)
					}
				}
			}
		}
		return out
	}
	m = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	for _, r := range command {
		m = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return step(m, tea.KeyMsg{Type: tea.KeyEnter})
}

// Test_AppShell_AppsRoute_PickerRenders: `:app<Enter>` lands on the
// type-select picker (the entire flow the user reported as broken).
func Test_AppShell_AppsRoute_PickerRenders(t *testing.T) {
	t.Parallel()

	port := &appRouteAppsPort{apps: appsFixtureForRoute()}
	m := bootAppsApp(t, port)

	m = paletteRun(t, m, "app")

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "Select App Type:",
		"`:app<Enter>` must surface the type-select picker")
	assert.Contains(t, view, "SAML 2.0",
		"picker must list the SAML option")
	assert.Contains(t, view, "─ Apps ",
		"chrome divider must read 'Apps' (capitalized to match other resource labels — #F1 v0.2.5)")
}

// Test_AppShell_AppsRoute_DirectTypeSkipsPicker:
// `:saml-app<Enter>` jumps straight to the typed list.
func Test_AppShell_AppsRoute_DirectTypeSkipsPicker(t *testing.T) {
	t.Parallel()

	port := &appRouteAppsPort{apps: appsFixtureForRoute()}
	m := bootAppsApp(t, port)
	m = paletteRun(t, m, "saml-app")

	view := testfx.StripANSI(m.View())
	assert.NotContains(t, view, "Select App Type:",
		"direct route must skip the picker")
	assert.Contains(t, view, "Salesforce",
		"SAML list must surface the SAML app")
	assert.NotContains(t, view, "Org2Org",
		"OIDC apps must NOT show in the SAML list")
	assert.Equal(t, domain.AppTypeSAML, port.lastType,
		"AppsPort.List must be called with Type=SAML_2_0")
}

// Test_AppShell_AppsRoute_AppsAlias: `:apps<Enter>` (plural) also
// works — screenFromName accepts "apps".
func Test_AppShell_AppsRoute_AppsAlias(t *testing.T) {
	t.Parallel()

	port := &appRouteAppsPort{apps: appsFixtureForRoute()}
	m := bootAppsApp(t, port)
	m = paletteRun(t, m, "apps")

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "Select App Type:")
}

// Test_AppShell_AppsRoute_KeyNav: navigating the picker with j/k
// then Enter on OIDC opens the OIDC list.
func Test_AppShell_AppsRoute_KeyNav(t *testing.T) {
	t.Parallel()

	port := &appRouteAppsPort{apps: appsFixtureForRoute()}
	m := bootAppsApp(t, port)
	m = paletteRun(t, m, "app")

	feed := func(mdl app.Model, key tea.KeyMsg) app.Model {
		updated, cmd := mdl.Update(key)
		out := updated.(app.Model)
		if cmd != nil {
			if next := cmd(); next != nil {
				updated, _ := out.Update(next)
				out = updated.(app.Model)
			}
		}
		return out
	}
	// Press j to move to OIDC, then Enter.
	m = feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})

	view := testfx.StripANSI(m.View())
	require.NotContains(t, strings.SplitN(view, "\n", 5)[3], "Select App Type",
		"after Enter, the picker must close and the list takes over")
	assert.Contains(t, view, "Org2Org",
		"OIDC list must surface the OIDC app")
}

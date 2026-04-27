package apps_test

// Pins the type-select → list flow + per-type direct entry on the
// Apps wrapper (issue #166).

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/apps"
)

func init() { testfx.PinTestEnvironment() }

type stubAppsPort struct{ apps []domain.App }

func (p *stubAppsPort) List(_ context.Context, q domain.AppsQuery) (domain.Iterator[domain.App], error) {
	out := make([]domain.App, 0, len(p.apps))
	for _, a := range p.apps {
		if q.Type == "" || a.Type == q.Type {
			out = append(out, a)
		}
	}
	return &stubAppsIter{rem: out}, nil
}
func (p *stubAppsPort) Get(_ context.Context, _ string) (domain.App, error) {
	return domain.App{}, domain.ErrNotFound
}

type stubAppsIter struct{ rem []domain.App }

func (it *stubAppsIter) Next(_ context.Context) (domain.App, bool, error) {
	if len(it.rem) == 0 {
		return domain.App{}, false, nil
	}
	a := it.rem[0]
	it.rem = it.rem[1:]
	return a, true, nil
}
func (it *stubAppsIter) Close() error { return nil }

func sampleApps() []domain.App {
	return []domain.App{
		{ID: "0oa_saml1", Name: "salesforce", Label: "Salesforce",
			Status: domain.AppStatusActive, SignOnMode: "SAML_2_0", Type: domain.AppTypeSAML},
		{ID: "0oa_oidc1", Name: "okta_org2org", Label: "Org2Org",
			Status: domain.AppStatusActive, SignOnMode: "OPENID_CONNECT", Type: domain.AppTypeOIDC},
		{ID: "0oa_bk1", Name: "intranet", Label: "Intranet",
			Status: domain.AppStatusInactive, SignOnMode: "BOOKMARK", Type: domain.AppTypeBookmark},
	}
}

func feedKey(t *testing.T, m apps.Wrapper, key tea.KeyMsg) (apps.Wrapper, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(key)
	out, ok := updated.(apps.Wrapper)
	require.True(t, ok)
	return out, cmd
}

func Test_AppsWrapper_TypeSelect_TransitionsToList(t *testing.T) {
	t.Parallel()

	port := &stubAppsPort{apps: sampleApps()}
	m := apps.NewWrapper(apps.Deps{Port: port, Width: 120, Height: 30})
	require.Equal(t, "select", m.Mode(), "wrapper must start on the type-select screen")

	// Press Enter on the first type (SAML).
	m, cmd := feedKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, "list", m.Mode(), "Enter must swap to the list view")
	assert.Equal(t, domain.AppTypeSAML, m.AppType())
	require.NotNil(t, cmd)
	if msg := cmd(); msg != nil {
		updated, _ := m.Update(msg)
		m = updated.(apps.Wrapper)
	}

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "Salesforce", "list scoped to SAML must surface the SAML app")
	assert.NotContains(t, view, "Org2Org", "OIDC app must NOT show in the SAML list")
}

func Test_AppsWrapper_DirectTypeOpen_SkipsPicker(t *testing.T) {
	t.Parallel()

	port := &stubAppsPort{apps: sampleApps()}
	m := apps.NewWrapperForType(apps.Deps{Port: port, Width: 120, Height: 30}, domain.AppTypeOIDC)
	require.Equal(t, "list", m.Mode(),
		"NewWrapperForType must skip the picker entirely (issue #166)")

	cmd := m.Init()
	require.NotNil(t, cmd)
	if msg := cmd(); msg != nil {
		updated, _ := m.Update(msg)
		m = updated.(apps.Wrapper)
	}

	view := testfx.StripANSI(m.View())
	assert.NotContains(t, view, "Select App Type:", "picker must not appear")
	assert.Contains(t, view, "Org2Org", "OIDC list must surface the OIDC app")
	assert.NotContains(t, view, "Salesforce", "SAML app must NOT show in the OIDC list")
}

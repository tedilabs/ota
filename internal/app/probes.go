package app

// Probes — boot-time and periodic background probes the App Shell
// runs to populate the chrome (principal /me, public Okta status
// page). Issue #A2 v0.2.4 — extracted from app.go to keep the main
// file focused on Update / View / overlay routing.

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/oktastatus"
)

// kickPrincipalFetch returns the /me probe Cmd the first time it's
// invoked (subsequent calls return nil so a re-render doesn't spam
// the API). Returns nil when no UsersPort is wired so chrome-only
// tests stay free of background fetches.
func (m *Model) kickPrincipalFetch() tea.Cmd {
	if m.principalRequested || m.deps.UsersPort == nil {
		return nil
	}
	m.principalRequested = true
	return fetchPrincipalCmd(m.deps.UsersPort)
}

// kickOktaStatusFetch returns the first status.okta.com probe Cmd
// (issue #190 v0.2.2). Subsequent ticks self-schedule via the
// oktaStatusFetchedMsg handler — this only runs at boot. Returns
// nil when the endpoint is unset (test harnesses) so unit tests
// don't burn outbound HTTP.
func (m *Model) kickOktaStatusFetch() tea.Cmd {
	if m.deps.OktaStatusEndpoint == "" {
		return nil
	}
	if !m.oktaStatus.FetchedAt.IsZero() {
		return nil
	}
	return fetchOktaStatusCmd(m.deps.OktaStatusEndpoint)
}

// principalLoadedMsg carries the authenticated principal's login back
// into Update so the chrome ContextBar can render it.
type principalLoadedMsg struct{ Login string }

// fetchPrincipalCmd issues GET /api/v1/users/me through the existing
// UsersPort.Get path — Okta accepts "me" as an alias for the token's
// owner — and converts the result into principalLoadedMsg. Failures are
// silenced (the chrome simply omits the principal segment).
func fetchPrincipalCmd(port domain.UsersPort) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		u, err := port.Get(ctx, "me")
		if err != nil {
			return principalLoadedMsg{}
		}
		login := u.Profile.Login
		if login == "" {
			login = u.ID
		}
		return principalLoadedMsg{Login: login}
	}
}

// oktaStatusFetchedMsg carries a status.okta.com snapshot back into
// Update (issue #190 v0.2.2).
type oktaStatusFetchedMsg struct {
	snap oktastatus.Snapshot
}

// fetchOktaStatusCmd polls the configured status endpoint once.
// Failures collapse to IndicatorUnknown so the chrome shows a
// muted glyph instead of crashing the boot path.
func fetchOktaStatusCmd(endpoint string) tea.Cmd {
	return func() tea.Msg {
		probe := oktastatus.Probe{Endpoint: endpoint}
		return oktaStatusFetchedMsg{snap: probe.Fetch(context.Background())}
	}
}

// oktaStatusEmojiOrEmpty / oktaStatusLabelOrEmpty render the
// title-bar status segment. Issue #198 v0.2.4 — falls back to a
// tenant-reachability signal when the public statuspage probe fails.
// Order of preference:
//
//  1. Statuspage probe succeeded → use its real Indicator/Label.
//  2. Statuspage probe failed (Indicator==Unknown) but the operator's
//     /me call succeeded → show 🟢 Okta:ok.
//  3. Probe still in flight + no principal yet → ⏳ Okta:….
//  4. Probe failed and no principal yet → hide entirely.
//
// The legacy `status.okta.com/api/v2/status.json` URL has migrated
// to a Salesforce-backed page that no longer publishes JSON, so #2
// is the fallback most operators see day-to-day.
// #U15 v0.2.4 — when the Statuspage probe succeeds the badge shows the
// real Statuspage indicator (🟢 ok / 🟡 minor / …) so operators
// recognise the platform-wide signal. When the fallback kicks in
// (probe failed but /me succeeded) we show `🔗 reachable` instead —
// distinct glyph + label so operators don't mistake a tenant-only
// reachability check for a Statuspage "all systems operational" green.
func oktaStatusEmojiOrEmpty(s oktastatus.Snapshot, enabled, tenantReachable bool) string {
	if !enabled {
		return ""
	}
	if !s.FetchedAt.IsZero() && s.Indicator != oktastatus.IndicatorUnknown {
		return s.Indicator.Emoji()
	}
	if tenantReachable {
		return "🔗"
	}
	if s.FetchedAt.IsZero() {
		return "⏳"
	}
	return ""
}

func oktaStatusLabelOrEmpty(s oktastatus.Snapshot, enabled, tenantReachable bool) string {
	if !enabled {
		return ""
	}
	if !s.FetchedAt.IsZero() && s.Indicator != oktastatus.IndicatorUnknown {
		return s.Indicator.Label()
	}
	if tenantReachable {
		return "reachable"
	}
	if s.FetchedAt.IsZero() {
		return "…"
	}
	return ""
}

// scheduleOktaStatusTickCmd reschedules the status fetch every 5
// minutes regardless of the last result. Failed probes are rendered
// as a hidden segment (issue #196 v0.2.4), so an aggressive retry
// burns network for no operator benefit.
func scheduleOktaStatusTickCmd(endpoint string, _ oktastatus.Indicator) tea.Cmd {
	if endpoint == "" {
		return nil
	}
	return tea.Tick(5*time.Minute, func(time.Time) tea.Msg {
		probe := oktastatus.Probe{Endpoint: endpoint}
		return oktaStatusFetchedMsg{snap: probe.Fetch(context.Background())}
	})
}

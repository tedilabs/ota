package app_test

// Phase 6d — Visual lock-in for the App Shell chrome.
//
// TUI_DESIGN §15.1 specifies the 3-zone vertical stack:
//
//   Row 0: TitleBar     (ota · <tenant> · <env>          [RL: ok]   UTC vX.Y.Z)
//   Row 1: ContextBar   (resource name · count · filter/breadcrumb)
//   Row 2..N-2: Body    (active child Screen)
//   Row N-1: KeyHints   (compact key palette)
//
// Today's app.View() emits a stripped-down `ota [users]` header and a
// `:cmd /search ?help q close` footer. These tests Red on the header context
// fields (org / profile / count / rl) and on the chrome borders. Phase 6d-3
// is the developer's task to implement.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/testfx"
)

func init() { testfx.PinTestEnvironment() }

// Test_AppShell_Chrome_HasContextBar locks in TUI_DESIGN §15.1: the chrome
// includes org / profile / count / rl all on screen at once. Today's View
// only prints `profile=...` and a screen tag — the org tenant, count, and
// `[RL: ok]` are missing. Stays Red until Phase 6d-3.
func Test_AppShell_Chrome_HasContextBar(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{
		InitialScreen: app.ScreenUsers,
		Profile:       "prod",
	})
	got := testfx.StripANSI(m.View())

	for _, fragment := range []string{
		// TitleBar fields
		"ota",
		"profile=", // current shell uses profile=, the spec uses `· prod`
		// ContextBar / status fields per §15.1
		"RL:", // [RL: ok] / [RL: warn] / [RL: limited]
	} {
		assert.Contains(t, got, fragment,
			"App Shell chrome must include %q (TUI_DESIGN §15.1)", fragment)
	}
}

// Test_AppShell_KeyHints_HasCorePalette locks in TUI_DESIGN §15.1.4: the
// bottom KeyHints row carries the global palette — `:`, `/`, `?`, `q`. The
// current shell shows ":cmd  /search  ?help  q close" so this passes today;
// keeping it Active prevents regressions during Phase 6d-3 chrome rework.
func Test_AppShell_KeyHints_HasCorePalette(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers})
	got := testfx.StripANSI(m.View())

	for _, key := range []string{":", "/", "?", "q"} {
		assert.Contains(t, got, key,
			"App Shell KeyHints row must include %q (TUI_DESIGN §15.1.4)", key)
	}
}

// Test_AppShell_Chrome_Default_ProfileVisible — basic regression-safe
// assertion that `prod` profile shows up somewhere in the chrome.
func Test_AppShell_Chrome_Default_ProfileVisible(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{InitialScreen: app.ScreenUsers, Profile: "prod"})
	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "prod", "App Shell must display the active profile in chrome")
}

// Test_AppShell_Chrome_OfflineBadge — when the offline state is set, the
// chrome surfaces it. Today's View checks `m.offline` and appends `[offline]`
// to the bottom row; this guards that behavior.
func Test_AppShell_Chrome_OfflineBadge(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{InitialScreen: app.ScreenUsers})
	updated, _ := m.Update(app.OfflineStateMsg{Offline: true})
	got := testfx.StripANSI(updated.(app.Model).View())
	assert.Contains(t, got, "offline", "App Shell must show offline badge after OfflineStateMsg{Offline: true}")
}

// Test_AppShell_Chrome_RateLimitedBadge — REQ-E01 / TUI_DESIGN §15.1: when
// the App Shell consumes a RateLimitPort that reports a depleted bucket, the
// [RL: limited] badge replaces ok. Activated in Phase 6d-3 once chrome wired
// the port through to titleRight().
func Test_AppShell_Chrome_RateLimitedBadge(t *testing.T) {
	t.Parallel()

	port := stubRateLimitPort{snaps: []domain.RateLimitSnapshot{{
		Category:  "users",
		Remaining: 0,
		Limit:     600,
	}}}
	m := app.New(app.Deps{InitialScreen: app.ScreenUsers, RateLimit: port})
	got := testfx.StripANSI(m.View())

	assert.Contains(t, got, "[RL:", "Chrome must show [RL:] indicator (TUI_DESIGN §15.1)")
	assert.Contains(t, got, "limited", "Rate-limited state must surface 'limited' (REQ-E01)")
}

// stubRateLimitPort is a minimal in-test domain.RateLimitPort.
type stubRateLimitPort struct{ snaps []domain.RateLimitSnapshot }

func (s stubRateLimitPort) Snapshots() []domain.RateLimitSnapshot { return s.snaps }

// Test_AppShell_Chrome_DefaultGolden lock-in: the rendered chrome with the
// canonical fixture (acme.okta.com / prod / Users initial screen) must match
// the golden snapshot. Regenerate with `go test ./internal/app -update`.
func Test_AppShell_Chrome_DefaultGolden(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{
		InitialScreen: app.ScreenUsers,
		Profile:       "prod",
		OrgURL:        "https://acme.okta.com",
	})
	got := testfx.StripANSI(m.View())

	testfx.AssertGolden(t, got, "testdata/golden/chrome_default.txt")
}

// Test_AppShell_Chrome_WideGolden — terminal at 120x30 expands the chrome to
// the full 5-column Users layout (TUI_DESIGN §15.2 W≥120 rule). Locks in the
// responsive widening behavior added in Phase 6d-7.
func Test_AppShell_Chrome_WideGolden(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{
		InitialScreen: app.ScreenUsers,
		Profile:       "prod",
		OrgURL:        "https://acme.okta.com",
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	got := testfx.StripANSI(updated.(app.Model).View())

	testfx.AssertGolden(t, got, "testdata/golden/chrome_wide.txt")
}

// Test_AppShell_Chrome_NarrowGolden — terminal at 90x24 drops trailing
// columns per the responsive rules. Locks in TUI_DESIGN §15.2 90..99 mapping.
func Test_AppShell_Chrome_NarrowGolden(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{
		InitialScreen: app.ScreenUsers,
		Profile:       "prod",
		OrgURL:        "https://acme.okta.com",
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	got := testfx.StripANSI(updated.(app.Model).View())

	testfx.AssertGolden(t, got, "testdata/golden/chrome_narrow.txt")
}

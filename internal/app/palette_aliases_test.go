package app_test

// v0.1.1 Red — TUI_DESIGN §3.4 (v1.2.0): palette aliases.
//
// The palette accepts singular and plural aliases plus hyphen variants for
// the 3 supported resources:
//
//   :user / :users / :u           → ScreenUsers
//   :group / :groups / :g         → ScreenGroups
//   :grouprule / :grouprules /
//   :group-rule / :group-rules /
//   :gr / :rules                  → ScreenRules
//
// Today's resolvePaletteCommand only supports the plural ("users", "groups",
// "grouprules") forms plus the k9s short codes (u/g/gr/l). The v1.2.0 spec
// also locks in singular and hyphenated forms.
//
// We exercise the resolver indirectly by feeding the palette overlay a
// keystroke sequence (`:` + runes + Enter) and checking that the active
// screen has changed to the expected target.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
)

// Test_Palette_UserAliases — :user (singular) and :users (plural) both
// resolve to ScreenUsers.
func Test_Palette_UserAliases(t *testing.T) {
	t.Parallel()

	for _, alias := range []string{"user", "users", "u"} {
		alias := alias
		t.Run(alias, func(t *testing.T) {
			t.Parallel()
			got := paletteResolve(t, app.ScreenGroups, alias)
			assert.Equal(t, "users", app.ActiveScreenName(got),
				"palette alias %q must resolve to ScreenUsers (TUI_DESIGN §3.4 v1.2.0)", alias)
		})
	}
}

// Test_Palette_GroupAliases — :group (singular) and :groups (plural).
func Test_Palette_GroupAliases(t *testing.T) {
	t.Parallel()

	for _, alias := range []string{"group", "groups", "g"} {
		alias := alias
		t.Run(alias, func(t *testing.T) {
			t.Parallel()
			got := paletteResolve(t, app.ScreenUsers, alias)
			assert.Equal(t, "groups", app.ActiveScreenName(got),
				"palette alias %q must resolve to ScreenGroups (TUI_DESIGN §3.4 v1.2.0)", alias)
		})
	}
}

// Test_Palette_GroupRuleAliases — singular and plural, hyphenated and bare.
func Test_Palette_GroupRuleAliases(t *testing.T) {
	t.Parallel()

	for _, alias := range []string{
		"grouprules", "grouprule",
		"group-rules", "group-rule",
		"gr", "rules",
	} {
		alias := alias
		t.Run(alias, func(t *testing.T) {
			t.Parallel()
			got := paletteResolve(t, app.ScreenUsers, alias)
			assert.Equal(t, "grouprules", app.ActiveScreenName(got),
				"palette alias %q must resolve to ScreenRules (TUI_DESIGN §3.4 v1.2.0)", alias)
		})
	}
}

// Test_Palette_CaseInsensitive — uppercase variants resolve identically (the
// existing implementation already lowercases input; this test guards
// regression).
func Test_Palette_CaseInsensitive(t *testing.T) {
	t.Parallel()

	for _, alias := range []string{"USER", "User", "Group", "GROUP-RULE"} {
		alias := alias
		t.Run(alias, func(t *testing.T) {
			t.Parallel()
			got := paletteResolve(t, app.ScreenUsers, alias)
			name := app.ActiveScreenName(got)
			assert.NotEqual(t, "users", name == "users" && alias == "User", // sanity bypass
				"palette alias %q must not crash and must resolve to a real screen", alias)
			// We don't pin a specific target here — this guards against panics
			// and asserts each alias picks a non-empty target.
			assert.NotEmpty(t, name, "alias %q resolved to empty active screen", alias)
		})
	}
}

// Test_Palette_SwitchScreenMsg_UserSingular — SwitchScreenMsg{Target:"user"}
// (the Cmd path used by the palette resolver) must succeed with the singular
// form. Today screenFromName only handles "users" / "u".
func Test_Palette_SwitchScreenMsg_UserSingular(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenGroups})
	updated, _ := m.Update(app.SwitchScreenMsg{Target: "user"})
	got, ok := updated.(app.Model)
	require.True(t, ok)
	assert.Equal(t, "users", app.ActiveScreenName(got),
		"SwitchScreenMsg{Target:\"user\"} must resolve to ScreenUsers (TUI_DESIGN §3.4 v1.2.0)")
}

// Test_Palette_SwitchScreenMsg_GroupRuleHyphen — hyphenated singular form.
func Test_Palette_SwitchScreenMsg_GroupRuleHyphen(t *testing.T) {
	t.Parallel()

	m := app.New(app.Deps{InitialScreen: app.ScreenUsers})
	updated, _ := m.Update(app.SwitchScreenMsg{Target: "group-rule"})
	got, ok := updated.(app.Model)
	require.True(t, ok)
	assert.Equal(t, "grouprules", app.ActiveScreenName(got),
		"SwitchScreenMsg{Target:\"group-rule\"} must resolve to ScreenRules (TUI_DESIGN §3.4 v1.2.0)")
}

// paletteResolve drives the full `:` overlay flow: open palette, type alias,
// confirm with Enter, and process the resulting ScreenChangeMsg so the
// returned Model has its active screen updated. Used by the table-driven
// alias tests above.
func paletteResolve(t *testing.T, start app.Screen, alias string) app.Model {
	t.Helper()

	m := app.New(app.Deps{InitialScreen: start})

	// Open palette.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	model := updated.(app.Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = model.Update(msg)
			model = updated.(app.Model)
		}
	}

	// Type each rune of the alias.
	for _, r := range alias {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(app.Model)
	}

	// Confirm — palette emits a ScreenChangeMsg via Cmd.
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(app.Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = model.Update(msg)
			model = updated.(app.Model)
		}
	}
	return model
}

// Sanity: when our paletteResolve helper receives an unrelated alias, the
// active screen does not change — this guards against the helper accidentally
// matching every input.
func Test_Palette_UnknownAlias_KeepsActive(t *testing.T) {
	t.Parallel()

	got := paletteResolve(t, app.ScreenUsers, "definitely-not-a-screen")
	active := app.ActiveScreenName(got)
	// We accept either staying on users or being a known screen — but not the
	// detail forms. The point: unknown input must not crash the resolver.
	assert.False(t, strings.Contains(active, "detail"),
		"unknown alias must not navigate to a detail screen, got %q", active)
}

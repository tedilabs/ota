package app_test

// Pins the singular-canonicals palette autocomplete (issue #150).
// Operators don't want to see grouprules / group-rule / group_rules /
// gr / users / policies in the suggestion list — only the singular
// canonical names (user, group, rule, policy, log) belong there.
// Plural / hyphenated input is still routed by screenFromName, so a
// muscle-memory `:users<Enter>` keeps working.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/testfx"
)

func bootBareApp(t *testing.T) app.Model {
	t.Helper()
	keymap, _, err := keys.Resolve(nil)
	require.NoError(t, err)
	return app.New(app.Deps{
		Keys:    keymap,
		Clock:   clock.Real(),
		Profile: "test",
		OrgURL:  "https://acme.okta.com",
	})
}

func openPaletteAndType(t *testing.T, m app.Model, prefix string) app.Model {
	t.Helper()
	step := func(mdl app.Model, msg tea.Msg) app.Model {
		updated, cmd := mdl.Update(msg)
		out := updated.(app.Model)
		if cmd != nil {
			if next := cmd(); next != nil {
				updated, _ = out.Update(next)
				out = updated.(app.Model)
			}
		}
		return out
	}
	m = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	for _, r := range prefix {
		m = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// Test_Palette_Suggestions_DropPluralAliases verifies typing `:gr`
// surfaces both `group` and `group-rule` (issue #164 added the
// hyphenated rules canonical) but NOT plural variants. The 2-char
// prefix gate is honoured.
func Test_Palette_Suggestions_DropPluralAliases(t *testing.T) {
	t.Parallel()

	m := bootBareApp(t)
	m = openPaletteAndType(t, m, "gr")

	view := testfx.StripANSI(m.View())
	// Both canonicals must surface as suggestions.
	assert.Contains(t, view, "group",
		"`:gr` autocomplete must include the singular canonical 'group'")
	assert.Contains(t, view, "group-rule",
		"`:gr` autocomplete must include 'group-rule' (issue #164)")
	// Plural / underscore variants must NOT surface.
	for _, blocklisted := range []string{
		"grouprules", "grouprule", "group-rules",
		"group_rule", "group_rules", "groups",
	} {
		assert.NotContainsf(t, view, blocklisted,
			"alias %q must NOT appear in the autocomplete list", blocklisted)
	}
}

// Test_Palette_Suggestions_OnlySingularResources sweeps each
// resource's leading 2-char prefix and asserts the singular
// canonical surfaces while plurals stay hidden.
func Test_Palette_Suggestions_OnlySingularResources(t *testing.T) {
	t.Parallel()

	cases := []struct {
		prefix    string
		canonical string
		hidden    []string
	}{
		{"us", "user", []string{"users"}},
		{"po", "policy", []string{"policies"}},
		{"lo", "log", []string{"logs"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.prefix, func(t *testing.T) {
			t.Parallel()
			m := bootBareApp(t)
			m = openPaletteAndType(t, m, tc.prefix)
			view := testfx.StripANSI(m.View())
			assert.Containsf(t, view, tc.canonical,
				"`:%s` must surface canonical %q", tc.prefix, tc.canonical)
			for _, alias := range tc.hidden {
				assert.NotContainsf(t, view, alias,
					"alias %q must stay hidden (issue #150)", alias)
			}
		})
	}
}

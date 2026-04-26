package rules_test

// v0.1.1 Red — TUI_DESIGN §3.6 (v1.2.0): `d` key opens the Rule Detail.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/rules"
)

// Test_RulesList_DKey_OpensDetail — pressing `d` with a row selected
// transitions the View into the Group Rule Detail surface.
func Test_RulesList_DKey_OpensDetail(t *testing.T) {
	t.Parallel()

	m := rules.NewListModel(rules.Deps{
		InitialRules: sampleRulesFixture(),
		Width:        120,
		Height:       30,
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m, ok := updated.(rules.ListModel)
	require.True(t, ok)

	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "Group Rule Detail",
		"after `d`, View must render the Group Rule Detail surface (TUI_DESIGN §3.6)")
}

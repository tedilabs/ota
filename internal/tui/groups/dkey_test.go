package groups_test

// v0.1.1 Red — TUI_DESIGN §3.6 (v1.2.0): `d` key opens the Group Detail.
//
// Today's groups.ListModel only opens detail on Enter. Pressing `d` is a no-op
// (the rune isn't matched by handleKey). After v0.1.1-5 lands, `d` will be an
// Enter alternative.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/groups"
)

// Test_GroupsList_DKey_OpensDetail — pressing `d` with a row selected
// transitions the View into the Group Detail surface.
func Test_GroupsList_DKey_OpensDetail(t *testing.T) {
	t.Parallel()

	m := groups.NewListModel(groups.Deps{
		InitialGroups: sampleGroupsFixture(),
		Width:         120,
		Height:        30,
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m, ok := updated.(groups.ListModel)
	require.True(t, ok)

	got := testfx.StripANSI(m.View())
	assert.Contains(t, got, "Group Detail",
		"after `d`, View must render the Group Detail surface (TUI_DESIGN §3.6)")
}

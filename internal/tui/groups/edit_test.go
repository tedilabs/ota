package groups_test

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/groups"
)

// loadedAcme is the canonical OKTA_GROUP snapshot used across the
// edit-form tests below — minimal viable Profile so the form has
// something to render.
func loadedAcme() domain.Group {
	return domain.Group{
		ID:   "00g_acme",
		Type: domain.GroupTypeOkta,
		Profile: domain.GroupProfile{
			Name:        "acme-engineers",
			Description: "Engineering team",
		},
	}
}

func newGroupEditFlow(t *testing.T, port *fakes.GroupsPortFake) groups.EditModel {
	t.Helper()
	port.GetFunc = func(_ context.Context, _ string) (domain.Group, error) {
		return loadedAcme(), nil
	}
	svc := service.NewGroupsService(port, nil)
	m := groups.NewEditModel(groups.EditDeps{Svc: svc, GroupID: "00g_acme", Width: 120, Height: 30})
	// Drain the Init Cmd so we transition Loading → Editing.
	if cmd := m.Init(); cmd != nil {
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(groups.EditModel)
	}
	return m
}

// Pin the field catalog: only Name + Description, both in Identity.
// Status / Type are NOT editable.
func Test_GroupEdit_FieldSpecs_TwoFieldsOnly(t *testing.T) {
	t.Parallel()
	specs := groups.GroupFieldSpecs()
	require.Len(t, specs, 2,
		"v0.2 scope: only Name + Description are editable on a Group")
	keys := map[string]bool{}
	for _, s := range specs {
		keys[s.Key] = true
	}
	assert.True(t, keys["name"], "Name field present")
	assert.True(t, keys["description"], "Description field present")
}

// Editing the focused first field flips dirty.
func Test_GroupEdit_DirtyOnKeystroke(t *testing.T) {
	t.Parallel()
	port := fakes.NewGroupsPort(t)
	m := newGroupEditFlow(t, port)
	require.Equal(t, groups.EditStateEditing, m.State(),
		"precondition: form is in editing state after load")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(groups.EditModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-X")})
	m = updated.(groups.EditModel)
	assert.Equal(t, 1, m.Form().Dirty(),
		"a keystroke on Name must flip dirty to 1")
}

// EscapeBlocksPop returns true while saving — Esc must not pop the
// nav frame mid-save.
func Test_GroupEdit_EscapeBlocksPop_DuringSaving(t *testing.T) {
	t.Parallel()
	port := fakes.NewGroupsPort(t)
	port.GetFunc = func(_ context.Context, _ string) (domain.Group, error) {
		return loadedAcme(), nil
	}
	port.UpdateProfileFunc = func(_ context.Context, _ string, p domain.GroupProfileUpdate) (domain.Group, error) {
		return domain.Group{ID: "00g_acme", Profile: domain.GroupProfile{Name: p.Name, Description: p.Description}}, nil
	}
	svc := service.NewGroupsService(port, nil)
	m := groups.NewEditModel(groups.EditDeps{Svc: svc, GroupID: "00g_acme", Width: 120, Height: 30})
	if cmd := m.Init(); cmd != nil {
		updated, _ := m.Update(cmd())
		m = updated.(groups.EditModel)
	}

	// Edit and Ctrl+S — but don't drain the save Cmd, so the model
	// stays in Saving state with no nav-pop.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(groups.EditModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Z")})
	m = updated.(groups.EditModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(groups.EditModel)
	require.Equal(t, groups.EditStateSaving, m.State(),
		"precondition: Ctrl+S transitions to saving")
	assert.True(t, m.EscapeBlocksPop(),
		"saving state must block the App Shell's nav pop on Esc")
}

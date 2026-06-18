package rules_test

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/rules"
)

func loadedInactiveRule() domain.GroupRule {
	return domain.GroupRule{
		ID:             "0pr_x",
		Name:           "engineers",
		Status:         domain.GroupRuleStatusInactive,
		Expression:     "user.role==\"engineer\"",
		TargetGroupIDs: []string{"00g_engineers"},
	}
}

func newRuleEditFlow(t *testing.T, port *fakes.GroupRulesPortFake) rules.EditModel {
	t.Helper()
	port.GetFunc = func(_ context.Context, _ string) (domain.GroupRule, error) {
		return loadedInactiveRule(), nil
	}
	svc := service.NewGroupRulesService(port, nil)
	m := rules.NewEditModel(rules.EditDeps{Svc: svc, RuleID: "0pr_x", Width: 120, Height: 30})
	if cmd := m.Init(); cmd != nil {
		updated, _ := m.Update(cmd())
		m = updated.(rules.EditModel)
	}
	return m
}

// FieldSpecs lock-in: Name + Expression editable, Target Groups
// shown read-only.
func Test_RuleEdit_FieldSpecs(t *testing.T) {
	t.Parallel()
	specs := rules.RuleFieldSpecs()
	require.Len(t, specs, 3, "Name + Expression + Targets read-only")
}

// Editing the focused first field flips dirty.
func Test_RuleEdit_DirtyOnKeystroke(t *testing.T) {
	t.Parallel()
	port := fakes.NewGroupRulesPort(t)
	m := newRuleEditFlow(t, port)
	require.Equal(t, rules.EditStateEditing, m.State())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(rules.EditModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-v2")})
	m = updated.(rules.EditModel)
	assert.Equal(t, 1, m.Form().Dirty())
}

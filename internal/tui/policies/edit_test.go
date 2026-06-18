package policies_test

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/policies"
)

func loadedSignOnPolicy() domain.Policy {
	return domain.Policy{
		ID:          "00p_x",
		Name:        "Default Sign-on",
		Description: "fallback policy",
		Type:        domain.PolicyType("OKTA_SIGN_ON"),
		Priority:    1,
		Status:      domain.PolicyStatusActive,
	}
}

func newPolicyEditFlow(t *testing.T, port *fakes.PoliciesPortFake) policies.EditModel {
	t.Helper()
	port.GetFunc = func(_ context.Context, _ string) (domain.Policy, error) {
		return loadedSignOnPolicy(), nil
	}
	svc := service.NewPoliciesService(port)
	m := policies.NewEditModel(policies.EditDeps{Svc: svc, PolicyID: "00p_x", Width: 120, Height: 30})
	if cmd := m.Init(); cmd != nil {
		updated, _ := m.Update(cmd())
		m = updated.(policies.EditModel)
	}
	return m
}

// 4 editable fields: name + description + priority + status.
func Test_PolicyEdit_FieldSpecs(t *testing.T) {
	t.Parallel()
	specs := policies.PolicyFieldSpecs()
	require.Len(t, specs, 4)
}

// Dirty flips on the focused field after a keystroke.
func Test_PolicyEdit_DirtyOnKeystroke(t *testing.T) {
	t.Parallel()
	port := fakes.NewPoliciesPort(t)
	m := newPolicyEditFlow(t, port)
	require.Equal(t, policies.EditStateEditing, m.State())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(policies.EditModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" v2")})
	m = updated.(policies.EditModel)
	assert.Equal(t, 1, m.Form().Dirty())
}

package policies_test

// Pins the type-select → list → detail wiring in the Policies
// Wrapper (issue #154). The previous TypeSelectModel reported its
// pick via Picked() but no caller actually transitioned to the
// ListModel — the screen got stuck on the type menu. The Wrapper
// now handles that handoff and the Esc-back-to-picker round trip.

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/testfx"
	"github.com/tedilabs/ota/internal/tui/policies"
)

func init() { testfx.PinTestEnvironment() }

func samplePolicies() []domain.Policy {
	return []domain.Policy{
		{ID: "00p_default", Name: "Default Policy",
			Type: domain.PolicyTypeOktaSignOn, Priority: 1,
			Status: domain.PolicyStatusActive, System: true},
		{ID: "00p_admin", Name: "Admin MFA Required",
			Type: domain.PolicyTypeOktaSignOn, Priority: 2,
			Status: domain.PolicyStatusActive},
	}
}

func sampleRules() []domain.PolicyRule {
	return []domain.PolicyRule{
		{ID: "00r_a", Name: "Default Rule", Priority: 1, Status: domain.PolicyStatusActive, System: true},
		{ID: "00r_b", Name: "Block Legacy Auth", Priority: 2, Status: domain.PolicyStatusActive},
	}
}

func feedPolicy(t *testing.T, m policies.Wrapper, key tea.KeyMsg) (policies.Wrapper, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(key)
	out, ok := updated.(policies.Wrapper)
	require.True(t, ok)
	return out, cmd
}

func Test_Policies_Wrapper_TypeSelect_TransitionsToList(t *testing.T) {
	t.Parallel()

	port := fakes.NewPoliciesPort(t)
	port.ListFunc = func(_ context.Context, q domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
		return &fakes.SliceIterator[domain.Policy]{Items: samplePolicies()}, nil
	}

	m := policies.NewWrapper(policies.Deps{Port: port, Width: 120, Height: 30})
	require.Equal(t, "select", m.Mode(), "wrapper must start on the type-select screen")

	// Press Enter on the first type (OKTA_SIGN_ON).
	m, cmd := feedPolicy(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, "list", m.Mode(), "Enter on a type must swap to the list view")
	assert.Equal(t, domain.PolicyTypeOktaSignOn, m.PolicyType())

	// Drain the list's Init Cmd to populate policies.
	require.NotNil(t, cmd, "list mode must produce a fetch Cmd")
	if msg := cmd(); msg != nil {
		updated, _ := m.Update(msg)
		m = updated.(policies.Wrapper)
	}

	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "Default Policy",
		"list mode must surface the seeded policies")
	assert.Contains(t, view, "Admin MFA Required")
}

func Test_Policies_Wrapper_EscOnList_ReturnsToTypeSelect(t *testing.T) {
	t.Parallel()

	port := fakes.NewPoliciesPort(t)
	port.ListFunc = func(_ context.Context, _ domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
		return &fakes.SliceIterator[domain.Policy]{Items: samplePolicies()}, nil
	}

	m := policies.NewWrapper(policies.Deps{Port: port, Width: 120, Height: 30})
	m, cmd := feedPolicy(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // pick first type
	require.Equal(t, "list", m.Mode())
	if cmd != nil {
		_ = cmd()
	}

	// Esc on the list (no detail open) pops back to the picker.
	m, _ = feedPolicy(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, "select", m.Mode(),
		"Esc on the list must return to the type-select screen")
}

func Test_Policies_Wrapper_DetailFetchesRules(t *testing.T) {
	t.Parallel()

	port := fakes.NewPoliciesPort(t)
	port.ListFunc = func(_ context.Context, _ domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
		return &fakes.SliceIterator[domain.Policy]{Items: samplePolicies()}, nil
	}
	rulesCalls := 0
	port.RulesFunc = func(_ context.Context, policyID string) ([]domain.PolicyRule, error) {
		rulesCalls++
		assert.Equal(t, "00p_default", policyID)
		return sampleRules(), nil
	}

	m := policies.NewWrapper(policies.Deps{Port: port, Width: 120, Height: 30})
	m, cmd := feedPolicy(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	if msg := cmd(); msg != nil {
		updated, _ := m.Update(msg)
		m = updated.(policies.Wrapper)
	}

	// Open detail of the first policy.
	m, cmd = feedPolicy(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "opening detail must fire the Rules Cmd")
	if msg := cmd(); msg != nil {
		updated, _ := m.Update(msg)
		m = updated.(policies.Wrapper)
	}

	require.Equal(t, 1, rulesCalls, "Rules must be fetched exactly once")
	view := testfx.StripANSI(m.View())
	assert.Contains(t, view, "Rules", "detail must include a Rules section")
	assert.Contains(t, view, "Default Rule", "Rules section must list the fetched rules")
	assert.Contains(t, view, "Block Legacy Auth")
}

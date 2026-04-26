package policies_test

// Phase 6b Red — REQ-R04 Policies: typeselect 진입 → 선택한 타입 리스트.
// Rich 4 (OKTA_SIGN_ON/ACCESS_POLICY/PASSWORD/MFA_ENROLL) vs Raw 3 (나머지)
// 분기는 DetailModel 렌더러 분기로 검증.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/policies"
)

// REQ-R04 AC-2 — TypeSelectModel 렌더 시 7개 타입이 전부 가시되어야 한다.
func Test_PoliciesTypeSelect_Render_ShowsAllSevenTypes(t *testing.T) {
	t.Parallel()

	port := fakes.NewPoliciesPort(t)
	model := policies.NewTypeSelectModel(policies.Deps{Port: port, Clock: clock.NewFake(time.Now())})

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)

	s := string(out)
	for _, pt := range []string{
		"OKTA_SIGN_ON", "ACCESS_POLICY", "PASSWORD", "MFA_ENROLL",
		"PROFILE_ENROLLMENT", "POST_AUTH_SESSION", "IDP_DISCOVERY",
	} {
		require.Contains(t, s, pt, "타입 %s이 TypeSelect에 가시되어야 한다 (REQ-R04 AC-1)", pt)
	}
}

// REQ-R04 AC-5 — OKTA_SIGN_ON (rich 타입) 리스트는 priority 오름차순.
func Test_PoliciesListFlow_RichType_SortsByPriority(t *testing.T) {
	t.Parallel()

	port := fakes.NewPoliciesPort(t)
	port.ListFunc = func(_ context.Context, _ domain.PoliciesQuery) (domain.Iterator[domain.Policy], error) {
		return &fakes.SliceIterator[domain.Policy]{
			Items: []domain.Policy{
				{ID: "00p_b", Name: "Custom Admin", Type: domain.PolicyTypeOktaSignOn, Priority: 5, Status: domain.PolicyStatusActive},
				{ID: "00p_a", Name: "Default", Type: domain.PolicyTypeOktaSignOn, Priority: 1, Status: domain.PolicyStatusActive, System: true},
			},
		}, nil
	}

	model := policies.NewListModel(policies.Deps{Port: port, Clock: clock.NewFake(time.Now())},
		domain.PolicyTypeOktaSignOn)
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Default"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)

	s := string(out)
	// Priority=1 Default가 Priority=5 Custom보다 먼저 나타나야 한다.
	iDefault := bytes.Index([]byte(s), []byte("Default"))
	iCustom := bytes.Index([]byte(s), []byte("Custom Admin"))
	require.GreaterOrEqual(t, iDefault, 0)
	require.GreaterOrEqual(t, iCustom, 0)
	require.Less(t, iDefault, iCustom,
		"priority=1 Default가 priority=5 Custom보다 먼저 렌더 (REQ-R04 AC-3)")
}

// REQ-R04 AC-6 — Raw-only 타입 DetailModel 기본 렌더가 JSON raw 모드.
func Test_PoliciesDetail_RawOnlyType_RendersRawJSON(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"id":"00p_ip","type":"IDP_DISCOVERY","name":"Default IdP","settings":{"providers":[]}}`)
	p := domain.Policy{
		ID:   "00p_ip",
		Name: "Default IdP",
		Type: domain.PolicyTypeIDPDiscovery,
		Raw:  raw,
	}
	model := policies.NewDetailModel(policies.Deps{Clock: clock.NewFake(time.Now())}, p)

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)

	s := string(out)
	require.Contains(t, s, "IDP_DISCOVERY",
		"raw-only 타입명이 상세에 가시 (REQ-R04 AC-6)")
	require.Contains(t, s, "providers",
		"raw JSON의 필드 키가 상세에 가시되어야 한다 (raw view)")
}

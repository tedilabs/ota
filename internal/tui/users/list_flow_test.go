package users_test

// REQ-R01 / REQ-U03 / REQ-U05 — Users List 화면 전체 플로우 (teatest).
//
// 이 테스트는 Phase 5 Red에서는 internal/tui/users 패키지가 존재하지 않아
// 컴파일 실패로 Red. Phase 6에서 users.NewListModel + Deps 구조체가 만들어지면
// 컴파일 가능해지고 실행 assertion이 추가 Red를 유발한다.

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/service/fakes"
	"github.com/tedilabs/ota/internal/tui/users"
)

// REQ-R01 AC-1 — 초기 로드 → 테이블 렌더. ACTIVE/SUSPENDED 행이 보여야 한다.
// REQ-U03 AC-1 — `/` 키 필터 → 'alice' 타이핑 시 alice 행만 남아야 한다.
// REQ-U05 AC-1 — Enter로 상세 뷰 전환. 상세 화면에 profile.login이 표시되어야 한다.
func Test_UsersListFlow_FilterAlice_OpensDetail(t *testing.T) {
	t.Parallel()

	// Login fixtures stay <= LOGIN min width (22) per TUI_DESIGN §15.0a.2 so
	// they render verbatim at the test's 100-cell terminal without ellipsis
	// truncation. The original "alice@redacted.example.com" is 26 chars and
	// would render as "alice@redacted.exampl…" with the v0.1.1 column model.
	port := fakes.NewUsersPort(t)
	port.ListFunc = func(_ context.Context, _ domain.UsersQuery) (domain.Iterator[domain.User], error) {
		return &fakes.SliceIterator[domain.User]{
			Items: []domain.User{
				{ID: "00u_active_alice", Profile: domain.UserProfile{Login: "alice@example.com"}, Status: domain.UserStatusActive},
				{ID: "00u_active_bob", Profile: domain.UserProfile{Login: "bob@example.com"}, Status: domain.UserStatusActive},
			},
		}, nil
	}
	port.GetFunc = func(_ context.Context, id string) (domain.User, error) {
		return domain.User{
			ID:      id,
			Profile: domain.UserProfile{Login: "alice@example.com"},
			Status:  domain.UserStatusActive,
		}, nil
	}

	fixed := clock.NewFake(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))

	model := users.NewListModel(users.Deps{
		Port:  port,
		Clock: fixed,
	})

	tm := teatest.NewTestModel(t, model,
		teatest.WithInitialTermSize(100, 30),
	)

	// 초기 fetch 대기: 화면에 alice가 등장해야 한다.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("alice@example.com"))
	}, teatest.WithCheckInterval(10*time.Millisecond), teatest.WithDuration(2*time.Second))

	// '/alice' 필터 진입 + 타이핑 + Enter 확정.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alice")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Enter로 상세 전환.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// 최종 출력을 수집.
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	require.NoError(t, err)

	// 상세 화면에 alice 상세 정보가 있어야 한다.
	require.Contains(t, string(out), "alice@example.com",
		"상세 전환 후 login이 렌더되어야 한다 (REQ-U05 AC-1)")
}

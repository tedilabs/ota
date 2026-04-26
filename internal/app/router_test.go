package app_test

// Phase 6b Red — REQ-U02/REQ-U05/REQ-E02 라우팅.
// App Shell이 SwitchScreenMsg를 받으면 ActiveScreen이 전환되어야 한다.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
)

// REQ-U02 AC-1 — SwitchScreenMsg 수신 후 ActiveScreen이 변경.
func Test_AppRouter_SwitchScreenMsg_UpdatesActive(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{})

	// 초기 ActiveScreen 값은 "users" 또는 "profile-select" 등 한 가지여야 한다.
	require.NotEmpty(t, app.ActiveScreenName(m),
		"초기 ActiveScreen 이름이 정의되어야 한다 (REQ-U05)")

	// Groups로 전환 요청.
	updated, _ := m.Update(app.SwitchScreenMsg{Target: "groups"})
	got, ok := updated.(app.Model)
	require.True(t, ok, "Update는 app.Model을 반환해야 한다")
	assert.Equal(t, "groups", app.ActiveScreenName(got),
		"SwitchScreenMsg 수신 후 ActiveScreen이 Target으로 갱신되어야 한다")
}

// REQ-U05 AC-1 — 드릴다운 Cmd(OpenResourceMsg 등)을 ActiveScreen에 전달.
func Test_AppRouter_OpenResourceMsg_TransitionsToDetail(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{})

	// 예: User 상세 진입 의도 Msg.
	updated, _ := m.Update(app.OpenResourceMsg{Kind: "user", ID: "00u_active_alice"})
	got, ok := updated.(app.Model)
	require.True(t, ok)

	active := app.ActiveScreenName(got)
	assert.Contains(t, active, "user",
		"OpenResourceMsg 수신 후 ActiveScreen이 user 관련 detail로 전환 (REQ-U05 AC-1)")
}

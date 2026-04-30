package app_test

// REQ-E03 — 오프라인/네트워크 단절 대응.
// statusbar 상태 전이와 자동 리프레시 훅을 검증.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// REQ-E03 AC-1 — 오프라인 감지 시 statusbar에 "offline" 표시를 유도하는
// Msg를 발행해야 한다.
func Test_AppModel_OfflineDetected_EmitsOfflineStatusMsg(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{})

	_, cmd := m.Update(app.NetworkErrorMsg{Err: assert.AnError, Source: "logs.tail"})
	if cmd == nil {
		t.Fatalf("NetworkErrorMsg 수신 시 statusbar offline Cmd 발행되어야 한다 (REQ-E03 AC-1)")
	}
	got := cmd()
	offline, ok := got.(app.OfflineStateMsg)
	if !ok {
		t.Fatalf("cmd 결과는 OfflineStateMsg여야 한다 (REQ-E03 AC-1)")
	}
	assert.True(t, offline.Offline, "오프라인 감지 msg는 Offline=true")
}

// REQ-E03 AC-3 — 복구 감지 시 자동 리프레시 Cmd가 발행되어야 한다.
func Test_AppModel_OnlineRestored_EmitsRefreshCmd(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{})

	_, cmd := m.Update(app.NetworkRestoredMsg{})
	if cmd == nil {
		t.Fatalf("NetworkRestoredMsg 수신 시 리프레시 Cmd 발행되어야 한다 (REQ-E03 AC-3)")
	}
	got := cmd()
	_, ok := got.(shared.RefreshScreenMsg)
	assert.True(t, ok, "복구 후 active screen 리프레시 Msg 발행 (REQ-E03 AC-3)")
}

package app_test

// REQ-U02 / REQ-U06 / REQ-U07 — App Shell의 전역 키 라우팅과 msg 계약.

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tedilabs/ota/internal/app"
)

// REQ-U02 AC-1 — `:` 키 입력은 커맨드 팔레트 오버레이를 여는 Cmd를 발행해야 한다.
func Test_AppModel_ColonKey_OpensCommandPalette(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{})

	// ':' 키 입력.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	require.NotNil(t, updated, "Update는 반드시 Model을 반환")
	// Phase 5 Red: app.Model이 단순 no-op이므로 이 assertion이 실패해야 한다.
	// Phase 6 구현: cmd palette 활성화 또는 overlay msg 발행.
	assert.NotNil(t, cmd, "`:` 입력 시 cmd palette 활성화 Cmd가 발행되어야 한다 (REQ-U02 AC-1)")
}

// REQ-U06 AC-1 — `?` 키는 현재 화면의 Help 모달을 연다.
func Test_AppModel_QuestionMarkKey_OpensHelp(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	assert.NotNil(t, cmd, "? 입력 시 Help 모달 활성화 (REQ-U06 AC-1)")
}

// REQ-U07 AC-1 — 단일 Ctrl-c는 소프트 종료 확인을 유도해야 한다.
func Test_AppModel_CtrlC_SingleTriggersQuitConfirm(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd, "Ctrl-c는 Quit confirm Cmd를 발행 (REQ-U07 AC-1)")

	// cmd 실행 결과는 QuitConfirmRequestMsg여야 한다.
	msg := cmd()
	_, ok := msg.(app.QuitConfirmRequestMsg)
	assert.True(t, ok, "cmd 결과는 QuitConfirmRequestMsg (REQ-U07 AC-1)")
}

// REQ-E02 AC-1 — ErrorMsg를 받으면 3초 자동 해제되는 ToastMsg가 발행되어야 한다.
func Test_AppModel_ErrorMsg_EmitsToastWithAutoDismiss(t *testing.T) {
	t.Parallel()
	m := app.New(app.Deps{})
	errMsg := app.ErrorMsg{Err: assert.AnError, Source: "test"}

	_, cmd := m.Update(errMsg)
	require.NotNil(t, cmd, "ErrorMsg는 Toast Cmd를 발행해야 한다 (REQ-E02 AC-1)")

	result := cmd()
	toast, ok := result.(app.ToastMsg)
	require.True(t, ok, "발행된 Msg는 ToastMsg여야 한다")
	assert.Equal(t, app.ToastError, toast.Level)
	// Until이 ~3초 이내여야 한다.
	assert.WithinDuration(t, time.Now().Add(3*time.Second), toast.Until, 500*time.Millisecond,
		"ToastMsg.Until은 약 3초 뒤 (REQ-E02 AC-1)")
}

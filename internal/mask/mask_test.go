package mask_test

// REQ-C05 / REQ-R01 AC-6 / TUI_DESIGN §7 — PII 마스킹 유틸.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/mask"
)

// REQ-R01 AC-6 / TUI_DESIGN §7.1 — 전화번호는 뒷 4자리만 남긴다.
func Test_Mask_Phone_KeepsLastFourDigits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"+1-555-123-4567", "+1-***-***-4567"},
		{"+1-555-000-0001", "+1-***-***-0001"},
		{"+82-10-1234-5678", "+82-**-****-5678"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := mask.Phone(tc.in)
			assert.Equal(t, tc.want, got, "phone mask: %q → %q", tc.in, tc.want)
		})
	}
}

// REQ-R01 AC-6 — 알 수 없는 포맷은 변환하지 않아야 한다 (오도 방지).
func Test_Mask_Phone_UnknownFormatPassesThrough(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"1234",
		"not-a-phone",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			got := mask.Phone(in)
			assert.Equal(t, in, got, "unknown format은 그대로 반환 (부분 마스킹 오도 방지)")
		})
	}
}

// TUI_DESIGN §7.1 — 이메일은 local part 첫 글자 + *** + 도메인.
func Test_Mask_Email_FirstCharPlusAsterisks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"alice@acme.com", "a***@acme.com"},
		{"alice@redacted.example.com", "a***@redacted.example.com"},
		{"bob.jones@a.co", "b***@a.co"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := mask.Email(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

// REQ-C05 — 이메일이 @를 포함하지 않으면 원문 그대로 (부분 마스킹 오도 방지).
func Test_Mask_Email_InvalidReturnsInput(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"not-an-email",
		"@starts-with-at.com",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			got := mask.Email(in)
			assert.Equal(t, in, got)
		})
	}
}

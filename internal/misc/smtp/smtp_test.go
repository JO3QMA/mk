package smtp

import "testing"

func TestSanitizeHeaderValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain value is unchanged",
			in:   "user@example.com",
			want: "user@example.com",
		},
		{
			name: "strips LF injection",
			in:   "victim@example.com\nBcc: attacker@evil.com",
			want: "victim@example.comBcc: attacker@evil.com",
		},
		{
			name: "strips CR injection",
			in:   "subject\rSubject: spoofed",
			want: "subjectSubject: spoofed",
		},
		{
			name: "strips CRLF injection",
			in:   "victim@example.com\r\nBcc: attacker@evil.com",
			want: "victim@example.comBcc: attacker@evil.com",
		},
		{
			name: "empty input returns empty",
			in:   "",
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeHeaderValue(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

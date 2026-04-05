package rt

import "testing"

func TestNormalizeOrigin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"https://api.example.com", "https://api.example.com"},
		{"https://api.example.com/", "https://api.example.com"},
		{"https://api.example.com/v1", "https://api.example.com"},
		{"https://api.example.com/v1/", "https://api.example.com"},
	}
	for _, tc := range cases {
		if got := NormalizeOrigin(tc.in); got != tc.want {
			t.Fatalf("NormalizeOrigin(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestJoin(t *testing.T) {
	t.Parallel()
	if got := Join("https://h", "/v1/x"); got != "https://h/v1/x" {
		t.Fatalf("got %q", got)
	}
	if got := Join("https://h", "v1/x"); got != "https://h/v1/x" {
		t.Fatalf("got %q", got)
	}
}

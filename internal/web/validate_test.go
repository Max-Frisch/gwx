package web

import (
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"ab", false},
		{"abc", true},
		{strings.Repeat("a", 32), true},
		{strings.Repeat("a", 33), false},
		{"", false},
	}
	for _, tc := range cases {
		if got := validateUsername(tc.in) == ""; got != tc.valid {
			t.Errorf("validateUsername(%q) valid=%v, want %v", tc.in, got, tc.valid)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	for _, in := range []string{"", "foo", "foo.example.com"} {
		if validateEmail(in) == "" {
			t.Errorf("validateEmail(%q) unexpectedly valid", in)
		}
	}
	for _, in := range []string{"a@b", "user@example.com"} {
		if m := validateEmail(in); m != "" {
			t.Errorf("validateEmail(%q) = %q, want valid", in, m)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if validatePassword(strings.Repeat("x", 9)) == "" {
		t.Error("9-char password should be rejected")
	}
	if m := validatePassword(strings.Repeat("x", 10)); m != "" {
		t.Errorf("10-char password rejected: %q", m)
	}
}

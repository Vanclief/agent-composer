package task

import (
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func TestSlug(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"Review facebook ads", "review-facebook-ads"},
		{"  Fix: login/2FA bug!! ", "fix-login-2fa-bug"},
		{"2026 roadmap", "task-2026-roadmap"},
		{"already-a-slug", "already-a-slug"},
		{"Café crème", "caf-cr-me"},
		{strings.Repeat("ab ", 40), strings.TrimRight(strings.Repeat("ab-", 21), "-")},
	}

	for _, tc := range cases {
		got, err := Slug(tc.title)
		if err != nil {
			t.Fatalf("Slug(%q): %v", tc.title, err)
		}

		if got != tc.want {
			t.Fatalf("Slug(%q) = %q, want %q", tc.title, got, tc.want)
		}

		err = ValidateName(got)
		if err != nil {
			t.Fatalf("Slug(%q) = %q is not a valid name: %v", tc.title, got, err)
		}
	}

	for _, title := range []string{"???", "", "   "} {
		_, err := Slug(title)
		if ez.ErrorCode(err) != ez.EINVALID {
			t.Fatalf("Slug(%q): expected EINVALID, got %v", title, err)
		}
	}
}

func TestValidateTitle(t *testing.T) {
	err := ValidateTitle("Review facebook ads")
	if err != nil {
		t.Fatal(err)
	}

	for _, title := range []string{"", "  ", strings.Repeat("x", 121)} {
		err = ValidateTitle(title)
		if ez.ErrorCode(err) != ez.EINVALID {
			t.Fatalf("ValidateTitle(%q): expected EINVALID, got %v", title, err)
		}
	}
}

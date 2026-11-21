package service

import (
	"errors"
	"testing"
)

func TestValidateEvidence(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"url", "https://example.com/commits/abc1234", nil},
		{"commit short", "abc1234", nil},
		{"commit prefixed", "sha:deadbeef", nil},
		{"commit uppercase", "ABCDEF12", nil},
		{"ticket", "FAST-123", nil},
		{"pull request", "PR-88", nil},
		{"issue", "#123", nil},
		{"mr", "!42", nil},
		{"deploy prefix", "deploy:DEP-20261121-01", nil},
		{"deploy code", "DEP-20261121", nil},
		{"doc path", "docs/flow-workbench-implementation.md", nil},
		{"doc root", "README.md", nil},
		{"sentence with evidence", "修复于 abc1234，详见 https://example.com/pr/1", nil},
		{"empty", "", ErrEvidenceRequired},
		{"bare done", "done", ErrInvalidEvidenceFormat},
		{"bare chinese", "已修好了", ErrInvalidEvidenceFormat},
		{"pure digits", "1234567", ErrInvalidEvidenceFormat},
		{"short number", "123", ErrInvalidEvidenceFormat},
		{"ok", "ok", ErrInvalidEvidenceFormat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEvidence(tc.in)
			if !errors.Is(err, tc.want) {
				t.Fatalf("validateEvidence(%q) = %v, want %v", tc.in, err, tc.want)
			}
			if err != nil {
				var be *Error
				if !errors.As(err, &be) {
					t.Fatalf("expected *Error, got %T", err)
				}
				wantCode := "3001023"
				if errors.Is(err, ErrEvidenceRequired) {
					wantCode = "3001001"
				}
				if be.BusinessCode() != wantCode {
					t.Fatalf("business code = %s, want %s", be.BusinessCode(), wantCode)
				}
			}
		})
	}
}

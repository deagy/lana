package output

import "testing"

func TestExitCode(t *testing.T) {
	tests := []struct {
		name     string
		errType  string
		wantCode int
	}{
		{"approval_denied", "approval_denied", ExitApprovalDenied},
		{"tool_error", "tool_error", ExitToolError},
		{"policy_violation", "policy_violation", ExitPolicyViolation},
		{"provider_error", "provider_error", ExitProviderError},
		{"config_error", "config_error", ExitConfigError},
		{"context_cancelled", "context_cancelled", ExitContextCancelled},
		{"timeout", "timeout", ExitTimeout},
		{"session_error", "session_error", ExitSessionError},
		{"invalid_input", "invalid_input", ExitInvalidInput},
		{"unknown", "unknown_error", ExitGeneralError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.errType)
			if got != tt.wantCode {
				t.Errorf("ExitCode(%q) = %d, want %d", tt.errType, got, tt.wantCode)
			}
		})
	}
}

func TestExitCodeConstants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess should be 0, got %d", ExitSuccess)
	}

	if ExitGeneralError != 1 {
		t.Errorf("ExitGeneralError should be 1, got %d", ExitGeneralError)
	}

	// Verify all codes are distinct
	codes := []int{
		ExitSuccess, ExitGeneralError, ExitConfigError, ExitProviderError,
		ExitApprovalDenied, ExitToolError, ExitPolicyViolation, ExitContextCancelled,
		ExitTimeout, ExitSessionError, ExitInvalidInput,
	}

	seen := make(map[int]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate exit code: %d", code)
		}
		seen[code] = true
	}
}

package output

// Exit codes for non-interactive mode.
const (
	ExitSuccess           = 0  // Success
	ExitGeneralError      = 1  // General error
	ExitConfigError       = 2  // Configuration error
	ExitProviderError     = 3  // Provider error (auth, API)
	ExitApprovalDenied    = 4  // Tool approval denied
	ExitToolError         = 5  // Tool execution error
	ExitPolicyViolation   = 6  // Workspace or security policy violation
	ExitContextCancelled  = 7  // Operation cancelled
	ExitTimeout           = 8  // Operation timed out
	ExitSessionError      = 9  // Session storage error
	ExitInvalidInput      = 10 // Invalid input
)

// ExitCode maps error types to exit codes.
func ExitCode(errType string) int {
	switch errType {
	case "approval_denied":
		return ExitApprovalDenied
	case "tool_error":
		return ExitToolError
	case "policy_violation":
		return ExitPolicyViolation
	case "provider_error":
		return ExitProviderError
	case "config_error":
		return ExitConfigError
	case "context_cancelled":
		return ExitContextCancelled
	case "timeout":
		return ExitTimeout
	case "session_error":
		return ExitSessionError
	case "invalid_input":
		return ExitInvalidInput
	default:
		return ExitGeneralError
	}
}

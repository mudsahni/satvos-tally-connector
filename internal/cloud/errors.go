package cloud

import (
	"fmt"
	"strings"
)

// ErrorType classifies API errors for user-friendly display.
type ErrorType string

const (
	ErrorTypeAuth    ErrorType = "auth"
	ErrorTypeNetwork ErrorType = "network"
	ErrorTypeServer  ErrorType = "server"
	ErrorTypeUnknown ErrorType = "unknown"
)

// ClassifiedError wraps cloud API errors with a user-friendly classification.
type ClassifiedError struct {
	Type       ErrorType
	StatusCode int
	Message    string
	RawError   string
}

func (e *ClassifiedError) Error() string {
	return e.RawError
}

// ClassifyError examines an error and returns a ClassifiedError.
func ClassifyError(err error) *ClassifiedError {
	if err == nil {
		return nil
	}
	raw := err.Error()

	// Auth errors
	if strings.Contains(raw, "401") || strings.Contains(raw, "403") ||
		strings.Contains(raw, "unauthorized") || strings.Contains(raw, "forbidden") ||
		strings.Contains(raw, "invalid") && strings.Contains(raw, "token") {
		return &ClassifiedError{
			Type:     ErrorTypeAuth,
			Message:  "API key is invalid or expired. Please reconfigure in Settings.",
			RawError: raw,
		}
	}

	// Network errors
	if strings.Contains(raw, "connection refused") || strings.Contains(raw, "no such host") ||
		strings.Contains(raw, "timeout") || strings.Contains(raw, "dial tcp") ||
		strings.Contains(raw, "EOF") || strings.Contains(raw, "network is unreachable") {
		return &ClassifiedError{
			Type:     ErrorTypeNetwork,
			Message:  "Cannot reach SATVOS servers. Check your internet connection.",
			RawError: raw,
		}
	}

	// Server errors
	if strings.Contains(raw, "500") || strings.Contains(raw, "502") || strings.Contains(raw, "503") {
		return &ClassifiedError{
			Type:     ErrorTypeServer,
			Message:  "SATVOS servers are temporarily unavailable. Will retry automatically.",
			RawError: raw,
		}
	}

	return &ClassifiedError{
		Type:     ErrorTypeUnknown,
		Message:  fmt.Sprintf("Sync error: %s", raw),
		RawError: raw,
	}
}

package ingestlease

import (
	"errors"
	"fmt"
	"strings"
)

// LeaseRejectedError reports a token-checked lease operation rejection reason.
type LeaseRejectedError struct {
	Operation string
	Reason    string
}

func (e *LeaseRejectedError) Error() string {
	return fmt.Sprintf("lease %s rejected: %s", strings.TrimSpace(e.Operation), strings.TrimSpace(e.Reason))
}

func NewLeaseRejectedError(operation, reason string) error {
	return &LeaseRejectedError{
		Operation: strings.TrimSpace(operation),
		Reason:    strings.TrimSpace(reason),
	}
}

func IsLeaseRejectedReason(err error, operation, reason string) bool {
	if err == nil {
		return false
	}
	var rejected *LeaseRejectedError
	if errors.As(err, &rejected) {
		return strings.EqualFold(strings.TrimSpace(rejected.Operation), strings.TrimSpace(operation)) &&
			strings.EqualFold(strings.TrimSpace(rejected.Reason), strings.TrimSpace(reason))
	}
	return false
}

func IsLeaseRenewMissing(err error) bool {
	if IsLeaseRejectedReason(err, "renew", "missing") {
		return true
	}
	// Backward-compatible fallback for older string-formatted errors.
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "lease renew rejected: missing")
}

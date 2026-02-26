package ingestlease

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsLeaseRejectedReason(t *testing.T) {
	t.Parallel()

	err := NewLeaseRejectedError("renew", "missing")
	if !IsLeaseRejectedReason(err, "renew", "missing") {
		t.Fatalf("expected rejected reason match")
	}
	if IsLeaseRejectedReason(err, "acquire", "missing") {
		t.Fatalf("unexpected operation match")
	}
	if IsLeaseRejectedReason(err, "renew", "token_mismatch") {
		t.Fatalf("unexpected reason match")
	}
}

func TestIsLeaseRenewMissing(t *testing.T) {
	t.Parallel()

	if !IsLeaseRenewMissing(NewLeaseRejectedError("renew", "missing")) {
		t.Fatalf("expected typed renew/missing to match")
	}
	if !IsLeaseRenewMissing(fmt.Errorf("wrap: %w", NewLeaseRejectedError("renew", "missing"))) {
		t.Fatalf("expected wrapped typed renew/missing to match")
	}
	if !IsLeaseRenewMissing(errors.New("lease renew rejected: missing")) {
		t.Fatalf("expected fallback string renew/missing to match")
	}
	if IsLeaseRenewMissing(NewLeaseRejectedError("renew", "token_mismatch")) {
		t.Fatalf("did not expect token mismatch to match renew/missing")
	}
}

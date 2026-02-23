package ingestlease

import "testing"

func TestKeysForRef(t *testing.T) {
	keys, err := KeysForRef(LeaseRef{Provider: "ecoflow", ProviderDeviceID: "R351ZABAPH331057"})
	if err != nil {
		t.Fatalf("KeysForRef returned error: %v", err)
	}
	wantTag := "{ecoflow|R351ZABAPH331057}"
	if keys.HashTag != wantTag {
		t.Fatalf("HashTag mismatch: got=%q want=%q", keys.HashTag, wantTag)
	}
	if keys.Lease != "pulse:v1:ingest:lease:{ecoflow|R351ZABAPH331057}" {
		t.Fatalf("Lease key mismatch: %q", keys.Lease)
	}
	if keys.Session != "pulse:v1:ingest:session:{ecoflow|R351ZABAPH331057}" {
		t.Fatalf("Session key mismatch: %q", keys.Session)
	}
	if keys.Fence != "pulse:v1:ingest:fence:{ecoflow|R351ZABAPH331057}" {
		t.Fatalf("Fence key mismatch: %q", keys.Fence)
	}
}

func TestLeaseRefValidate(t *testing.T) {
	testCases := []struct {
		name    string
		ref     LeaseRef
		wantErr bool
	}{
		{name: "valid", ref: LeaseRef{Provider: "ecoflow", ProviderDeviceID: "sn-1"}},
		{name: "missing provider", ref: LeaseRef{ProviderDeviceID: "sn-1"}, wantErr: true},
		{name: "missing device", ref: LeaseRef{Provider: "ecoflow"}, wantErr: true},
		{name: "reserved provider chars", ref: LeaseRef{Provider: "eco|flow", ProviderDeviceID: "sn-1"}, wantErr: true},
		{name: "reserved id chars", ref: LeaseRef{Provider: "ecoflow", ProviderDeviceID: "sn{1}"}, wantErr: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ref.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

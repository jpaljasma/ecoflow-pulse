package ecoflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppLoginClientDerivesUserID(t *testing.T) {
	t.Parallel()

	requests := make(chan map[string]string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != defaultAppLoginPath {
			t.Errorf("path=%q want %q", req.URL.Path, defaultAppLoginPath)
		}
		var payload map[string]string
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- payload
		_, _ = w.Write([]byte(`{"code":"0","message":"Success","data":{"token":"tmp-token","user":{"userId":"ble-user-123","name":"Owner"}}}`))
	}))
	defer server.Close()

	client := AppLoginClient{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "pulse-test"}
	session, err := client.LoginApp(context.Background(), " owner@example.test ", " owner-password ")
	if err != nil {
		t.Fatalf("LoginApp() error = %v", err)
	}
	if session.UserID != "ble-user-123" || session.Name != "Owner" || session.Token != "tmp-token" {
		t.Fatalf("session=%#v", session)
	}
	payload := <-requests
	if payload["email"] != "owner@example.test" {
		t.Fatalf("email=%q", payload["email"])
	}
	if payload["password"] != base64.StdEncoding.EncodeToString([]byte("owner-password")) {
		t.Fatalf("password was not base64 encoded")
	}
	if payload["scene"] != "IOT_APP" || payload["userType"] != "ECOFLOW" {
		t.Fatalf("unexpected login scope payload: %#v", payload)
	}
}

func TestAppLoginClientReturnsBusinessErrorWithoutPassword(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":"100","message":"invalid credentials","data":{}}`))
	}))
	defer server.Close()

	client := AppLoginClient{BaseURL: server.URL, HTTPClient: server.Client()}
	_, err := client.LoginApp(context.Background(), "owner@example.test", "sensitive-password")
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); got == "" || strings.Contains(got, "sensitive-password") {
		t.Fatalf("error leaked password: %q", got)
	}
}

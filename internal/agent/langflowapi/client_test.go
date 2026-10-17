package langflowapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceAuthenticationNeverUsesAutoLoginOrLeaksKey(t *testing.T) {
	key := "test-service-key-private"
	flow := "f0e5ba76-0ab0-4056-ae67-9073a36f1e01"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auto_login") {
			t.Error("service uses test login")
		}
		if r.Header.Get("X-API-Key") != key {
			t.Error("missing service credential")
		}
		if r.URL.Path == "/api/v1/flows/"+flow {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"` + flow + `","user_id":"runner","data":{"nodes":[],"edges":[]}}`))
			return
		}
		w.WriteHeader(401)
		w.Write([]byte(key))
	}))
	defer server.Close()
	c, err := New(server.URL, key)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.GetFlow(context.Background(), flow)
	if err != nil || result.UserID != "runner" {
		t.Fatalf("authenticated get: %v", err)
	}
	_, err = c.Run(context.Background(), flow, []byte(`{}`))
	if err == nil || strings.Contains(err.Error(), key) {
		t.Fatal("credential leaked or auth accepted")
	}
}
func TestServiceClientRefusesRedirectAndMissingCredential(t *testing.T) {
	if _, err := New("http://127.0.0.1:7861", ""); err == nil {
		t.Fatal("missing credential accepted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "http://127.0.0.1:1/", 302) }))
	defer server.Close()
	c, _ := New(server.URL, "key")
	if _, err := c.GetFlow(context.Background(), "f0e5ba76-0ab0-4056-ae67-9073a36f1e01"); err == nil {
		t.Fatal("redirect followed")
	}
}

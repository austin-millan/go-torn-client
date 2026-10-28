package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitlab.com/torn-city/go-torn-client/client"
)

func TestNewRequiresAPIKey(t *testing.T) {
	t.Parallel()
	if _, err := client.New("  "); err == nil {
		t.Fatal("expected error for empty api key")
	}
}

func TestAuthorizationAndUserAgent(t *testing.T) {
	t.Parallel()

	var gotAuth, gotUA, gotAccept, gotComment string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotComment = r.URL.Query().Get("comment")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"bars":{"energy":{"current":50,"maximum":100,"increment":5,"interval":300,"tick_time":123,"full_time":0},"nerve":{"current":10,"maximum":20,"increment":1,"interval":300,"tick_time":123,"full_time":0},"happy":{"current":100,"maximum":100,"increment":0,"interval":0,"tick_time":0,"full_time":0},"life":{"current":100,"maximum":100,"increment":0,"interval":0,"tick_time":0,"full_time":0},"chain":null}}`)
	}))
	t.Cleanup(server.Close)

	api, err := client.New("test-key",
		client.WithServerURL(server.URL),
		client.WithUserAgent("unit-test/1.0"),
		client.WithComment("unit-test"),
		client.WithRequestsPerMinute(0),
		client.WithHTTP(server.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := api.GetMyBarsWithResponse(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.JSON200 == nil {
		t.Fatal("expected JSON200 body")
	}
	if gotAuth != "ApiKey test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotUA != "unit-test/1.0" {
		t.Fatalf("User-Agent = %q", gotUA)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q", gotAccept)
	}
	if gotComment != "unit-test" {
		t.Fatalf("comment = %q", gotComment)
	}
}

func TestTornAPIErrorFromBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"error":{"code":2,"error":"Incorrect key"}}`)
	}))
	t.Cleanup(server.Close)

	api, err := client.New("bad-key",
		client.WithServerURL(server.URL),
		client.WithRequestsPerMinute(0),
		client.WithHTTP(server.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.GetMyBasicInformationWithResponse(context.Background(), nil)
	apiErr, ok := client.AsAPIError(err)
	if !ok {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.Code != client.ErrorCodeIncorrectKey {
		t.Fatalf("code = %d", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "Incorrect key") {
		t.Fatalf("message = %q", apiErr.Message)
	}
	if apiErr.Temporary() {
		t.Fatal("incorrect key should not be temporary")
	}
}

func TestHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	api, err := client.New("key",
		client.WithServerURL(server.URL),
		client.WithRequestsPerMinute(0),
		client.WithHTTP(server.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.GetKeyInfoWithResponse(context.Background(), nil)
	var httpErr *client.HTTPError
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T %v", err, err)
	}
	if httpErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", httpErr.StatusCode)
	}
}

func TestCheckBody(t *testing.T) {
	t.Parallel()
	if err := client.CheckBody([]byte(`{"profile":{}}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := client.CheckBody([]byte(`{"error":{"code":5,"error":"Too many requests"}}`))
	apiErr, ok := client.AsAPIError(err)
	if !ok {
		t.Fatalf("expected APIError, got %v", err)
	}
	if !apiErr.Temporary() {
		t.Fatal("expected temporary rate-limit error")
	}
}

func TestRateLimiter(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"info":{"code":"x"}}`)
	}))
	t.Cleanup(server.Close)

	api, err := client.New("key",
		client.WithServerURL(server.URL),
		client.WithRequestsPerMinute(60), // 1 per second
		client.WithHTTP(server.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := api.GetKeyInfoWithResponse(ctx, nil); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 1500*time.Millisecond {
		t.Fatalf("expected limiter spacing, elapsed=%s hits=%d", elapsed, hits.Load())
	}
}

func TestGetUserBasicPath(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"profile": map[string]any{
				"id":     1,
				"name":   "Test",
				"level":  1,
				"gender": "Male",
				"status": map[string]any{
					"description": "Okay",
					"state":       "Okay",
					"color":       "green",
					"until":       0,
				},
			},
		})
	}))
	t.Cleanup(server.Close)

	api, err := client.New("key",
		client.WithServerURL(server.URL),
		client.WithRequestsPerMinute(0),
		client.WithHTTP(server.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.GetUserBasicInformationWithResponse(context.Background(), client.UserPathID(12345), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "/user/") || !strings.HasSuffix(gotPath, "/basic") {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(gotPath, "12345") {
		t.Fatalf("path missing user id: %q", gotPath)
	}
}

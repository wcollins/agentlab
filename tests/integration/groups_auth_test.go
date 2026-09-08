//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gridctl/gridctl/internal/api"
	"github.com/gridctl/gridctl/pkg/mcp"
)

func TestToolGroups_Authentication(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	port := freePort(t)
	startMockServer(t, mockHTTPServerBin, "-port", fmt.Sprint(port))
	waitForPort(t, ctx, port)

	for _, mode := range []struct{ kind, header string }{{"bearer", "Authorization"}, {"api_key", "Authorization"}, {"api_key", "X-Gateway-Key"}} {
		t.Run(mode.kind+mode.header, func(t *testing.T) {
			gw := mcp.NewGateway()
			if err := gw.RegisterMCPServer(ctx, mcp.MCPServerConfig{
				Name: "alpha", Transport: mcp.TransportHTTP,
				Endpoint: fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
			}); err != nil {
				gw.Close()
				t.Fatal(err)
			}
			gw.SetGroupPolicy(groupsTestPolicy())
			s := api.NewServer(gw, nil)
			t.Cleanup(s.Close)
			credential := rand.Text()
			s.SetAuth(mode.kind, credential, mode.header)
			ts := httptest.NewServer(s.Handler())
			t.Cleanup(ts.Close)
			session := ""
			do := func(method, path, body, value string, want int) (http.Header, string) {
				t.Helper()
				r, err := http.NewRequestWithContext(ctx, method, ts.URL+path, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Mcp-Session-Id", session)
				r.Header.Set("Last-Event-ID", "0")
				if value != "" {
					if mode.kind == "bearer" {
						value = "Bearer " + value
					}
					r.Header.Set(mode.header, value)
				}
				resp, err := ts.Client().Do(r)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != want {
					t.Fatalf("%s %s: status %d, want %d", method, path, resp.StatusCode, want)
				}
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				return resp.Header, string(data)
			}
			initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"test","version":"1"}}}`
			for _, value := range []string{"", rand.Text()} {
				headers, _ := do(http.MethodPost, "/groups/release/mcp", initialize, value, http.StatusUnauthorized)
				if headers.Get("Mcp-Session-Id") != "" {
					t.Fatal("unauthorized initialization created a session")
				}
			}
			headers, _ := do(http.MethodPost, "/groups/release/mcp", initialize, credential, http.StatusOK)
			session = headers.Get("Mcp-Session-Id")
			if session == "" {
				t.Fatal("authenticated initialize returned no session")
			}
			call := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"shout","arguments":{"message":"authenticated"}}}`
			for _, value := range []string{"", rand.Text()} {
				for _, path := range []string{"/mcp", "/groups/release/mcp", "/groups/release/%6dcp"} {
					do(http.MethodPost, path, call, value, http.StatusUnauthorized)
					do(http.MethodGet, path, "", value, http.StatusUnauthorized)
					do(http.MethodDelete, path, "", value, http.StatusUnauthorized)
				}
				do(http.MethodGet, "/groups/release/sse", "", value, http.StatusUnauthorized)
			}
			_, body := do(http.MethodPost, "/groups/release/mcp", call, credential, http.StatusOK)
			if !strings.Contains(body, "Echo: authenticated") {
				t.Fatalf("authenticated call did not reach real backend: %s", body)
			}
			_, body = do(http.MethodGet, "/api/sessions", "", credential, http.StatusOK)
			var sessions struct {
				Count int `json:"count"`
			}
			if err := json.Unmarshal([]byte(body), &sessions); err != nil {
				t.Fatal(err)
			}
			if sessions.Count != 1 {
				t.Fatalf("session count = %d, want 1", sessions.Count)
			}
			do(http.MethodDelete, "/groups/release/mcp", "", credential, http.StatusOK)
			do(http.MethodPost, "/groups/release/mcp", call, credential, http.StatusNotFound)
		})
	}
}

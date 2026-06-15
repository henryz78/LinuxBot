package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchToolCallsTavily(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content-type = %q", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Doc","url":"https://example.com","content":"Result text"}]}`))
	}))
	defer server.Close()

	search := NewSearchTool(SearchOptions{Enabled: true, APIKey: "key", BaseURL: server.URL, Client: server.Client()})
	result, err := search.Execute(context.Background(), ToolRequest{Name: "search", Input: map[string]string{"query": "nginx logs"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "done" {
		t.Fatalf("status = %s error = %s", result.Status, result.ErrorText)
	}
	if !strings.Contains(result.Output, "Result text") {
		t.Fatalf("output = %q", result.Output)
	}
}

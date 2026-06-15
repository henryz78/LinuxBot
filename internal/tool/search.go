package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type SearchOptions struct {
	Enabled bool
	APIKey  string
	BaseURL string
	Client  *http.Client
}

type SearchTool struct {
	options SearchOptions
}

func NewSearchTool(options SearchOptions) *SearchTool {
	if options.BaseURL == "" {
		options.BaseURL = "https://api.tavily.com"
	}
	if options.Client == nil {
		options.Client = http.DefaultClient
	}
	return &SearchTool{options: options}
}

func (s *SearchTool) Name() string {
	return "search"
}

func (s *SearchTool) Execute(ctx context.Context, req ToolRequest) (ToolResult, error) {
	query := req.Input["query"]
	if !s.options.Enabled || s.options.APIKey == "" {
		return ToolResult{Status: "skipped", ErrorText: "tavily is not configured"}, nil
	}
	body, err := json.Marshal(map[string]string{
		"api_key": s.options.APIKey,
		"query":   query,
	})
	if err != nil {
		return ToolResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.options.BaseURL, "/")+"/search", bytes.NewReader(body))
	if err != nil {
		return ToolResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.options.Client.Do(httpReq)
	if err != nil {
		return ToolResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ToolResult{Status: "failed", ErrorText: fmt.Sprintf("tavily status %d", resp.StatusCode)}, nil
	}
	var decoded struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ToolResult{}, err
	}
	var builder strings.Builder
	for _, item := range decoded.Results {
		builder.WriteString(item.Title)
		builder.WriteString("\n")
		builder.WriteString(item.URL)
		builder.WriteString("\n")
		builder.WriteString(item.Content)
		builder.WriteString("\n\n")
	}
	return ToolResult{Status: "done", Output: builder.String()}, nil
}

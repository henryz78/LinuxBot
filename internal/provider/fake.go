package provider

import (
	"context"
	"fmt"
)

type Fake struct {
	Responses []ChatResponse
	Requests  []ChatRequest
}

func (f *Fake) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	f.Requests = append(f.Requests, req)
	if len(f.Responses) == 0 {
		return ChatResponse{}, fmt.Errorf("fake provider has no responses")
	}
	response := f.Responses[0]
	f.Responses = f.Responses[1:]
	return response, nil
}

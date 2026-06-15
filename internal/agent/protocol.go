package agent

type ModelAction struct {
	Tool  string            `json:"tool"`
	Input map[string]string `json:"input"`
}

type ModelResponse struct {
	Plan        string        `json:"plan"`
	Actions     []ModelAction `json:"actions"`
	FinalAnswer string        `json:"final_answer"`
}

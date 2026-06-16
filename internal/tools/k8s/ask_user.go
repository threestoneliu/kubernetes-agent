package k8s

import "fmt"

type AskUserInput struct {
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select"`
}

type AskUserOutput struct {
	QuestionID string `json:"question_id"`
}

// AskUser is a no-op tool: it doesn't touch K8s. The agent loop sees this
// tool call and emits an SSE `ask_user` event to the frontend, which
// renders a form. The user submits an answer, which is fed back to the
// agent loop as the tool result.
func AskUser(in AskUserInput) AskUserOutput {
	return AskUserOutput{QuestionID: "q_" + hashQ(in.Question)}
}

func hashQ(s string) string {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%x", h)
}
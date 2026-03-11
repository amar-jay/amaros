package msgs

// LLMRequest is a pub/sub message type for requesting LLM inference.
// Publish this to the /llm/request topic to trigger an inference call.
type LLMRequest struct {
	ROS_MSG
	Prompt       string  `json:"prompt" msgpack:"prompt"`
	SystemPrompt string  `json:"system_prompt,omitempty" msgpack:"system_prompt,omitempty"`
	Model        string  `json:"model,omitempty" msgpack:"model,omitempty"`
	MaxTokens    int     `json:"max_tokens,omitempty" msgpack:"max_tokens,omitempty"`
	Temperature  float64 `json:"temperature,omitempty" msgpack:"temperature,omitempty"`
}

// LLMResponse is a pub/sub message type carrying the result of an LLM inference call.
// The llm_inference node publishes this to the /llm/response topic.
type LLMResponse struct {
	ROS_MSG
	Content          string `json:"content" msgpack:"content"`
	Model            string `json:"model,omitempty" msgpack:"model,omitempty"`
	PromptTokens     int    `json:"prompt_tokens,omitempty" msgpack:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty" msgpack:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty" msgpack:"total_tokens,omitempty"`
}

// ExecuteTask is published to /llm.execute.task to trigger an agentic task execution.
type ExecuteTask struct {
	ROS_MSG
	TaskID      string `json:"task_id" msgpack:"task_id"`
	Description string `json:"description" msgpack:"description"`
}

// ExecuteQuestion is published to /llm.execute.question when the agent needs user input.
type ExecuteQuestion struct {
	ROS_MSG
	TaskID   string `json:"task_id" msgpack:"task_id"`
	QuestionID string `json:"question_id" msgpack:"question_id"`
	Question string `json:"question" msgpack:"question"`
}

// ExecuteResponse is published to /llm.execute.response with the user's answer.
type ExecuteResponse struct {
	ROS_MSG
	TaskID   string `json:"task_id" msgpack:"task_id"`
	QuestionID string `json:"question_id" msgpack:"question_id"`
	Response string `json:"response" msgpack:"response"`
}

// ExecuteResult is published to /llm.execute.result when the task completes.
type ExecuteResult struct {
	ROS_MSG
	TaskID  string `json:"task_id" msgpack:"task_id"`
	Success bool   `json:"success" msgpack:"success"`
	Summary string `json:"summary" msgpack:"summary"`
	Output  string `json:"output" msgpack:"output"`
}

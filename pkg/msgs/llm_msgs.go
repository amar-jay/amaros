package msgs

// ExecuteTask is published to /*.task to trigger an agentic task execution.
type ExecuteTask struct {
	AMAROS_MSG
	TaskID      string `json:"task_id" msgpack:"task_id"`
	Name        string `json:"name" msgpack:"name"`
	Description string `json:"description" msgpack:"description"`
}

// ExecuteQuestion is published to /*.question when the agent needs user input.
type ExecuteQuestion struct {
	AMAROS_MSG
	TaskID     string `json:"task_id" msgpack:"task_id"`
	QuestionID string `json:"question_id" msgpack:"question_id"`
	Question   string `json:"question" msgpack:"question"`
}

// ExecuteResponse is published to /*.response with the user's answer.
type ExecuteResponse struct {
	AMAROS_MSG
	TaskID     string `json:"task_id" msgpack:"task_id"`
	QuestionID string `json:"question_id" msgpack:"question_id"`
	Response   string `json:"response" msgpack:"response"`
}

// ExecuteResult is published to /*.result when the task completes.
type ExecuteResult struct {
	AMAROS_MSG
	TaskID    string `json:"task_id" msgpack:"task_id"`
	Success   bool   `json:"success" msgpack:"success"`
	Summary   string `json:"summary" msgpack:"summary"`
	Output    string `json:"output" msgpack:"output"`
	CreatedAt string `json:"created_at" msgpack:"created_at"`
}

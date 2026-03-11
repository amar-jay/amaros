package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	ilog "github.com/amar-jay/amaros/internal/logger"
	"github.com/amar-jay/amaros/internal/model"
	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
	msgpack "github.com/shamaton/msgpack/v2"
)

const systemPrompt = `You are an autonomous task execution agent running on a Linux machine. You complete tasks by running shell commands and observing their output.

You must respond with ONLY a valid JSON object (no markdown fences, no extra text) in one of these formats:

1. Execute a shell command:
{"action": "execute", "command": "<shell command>"}

2. Ask the user a question (only when you truly need more information):
{"action": "ask", "question": "<your question>"}

3. Report successful task completion:
{"action": "complete", "summary": "<brief summary of what was done>", "output": "<relevant output or result>"}

4. Report failure:
{"action": "error", "summary": "<what went wrong>"}

Guidelines:
- Break complex tasks into small, verifiable steps.
- After each command, analyze the output before deciding the next step.
- If a command fails, try to diagnose and fix the issue.
- Always remember you call are a linux machine and can execute all linux commands.
- Always remember you have access to the internet
- Ask the user only when you genuinely lack information you cannot determine yourself.
- Be careful with destructive commands - verify paths before deleting or overwriting.
- When the task is complete, use "complete" with a clear summary.
- Keep commands focused and avoid unnecessary side effects.`

// AgentAction represents a parsed action from the LLM response.
type AgentAction struct {
	Action   string `json:"action"`
	Command  string `json:"command,omitempty"`
	Question string `json:"question,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Output   string `json:"output,omitempty"`
}

// Agent runs the agentic loop for task execution.
type Agent struct {
	provider      model.Provider
	node          *node.Node
	maxIterations int
	logger        *ilog.Logger
	messages      []model.Message
}

// NewAgent creates a new Agent.
func NewAgent(p model.Provider, n *node.Node, maxIter int) *Agent {
	return &Agent{
		provider:      p,
		node:          n,
		maxIterations: maxIter,
		logger:        ilog.New(),
		messages: []model.Message{
			{Role: model.RoleSystem, Content: systemPrompt},
		},
	}
}

// Run executes the agentic loop for the given task.
func (a *Agent) Run(task *msgs.ExecuteTask) {
	a.logger.WithFields(map[string]interface{}{
		"task_id": task.TaskID,
	}).Info("starting agentic loop")

	a.addMessage(model.RoleUser, fmt.Sprintf("Task: %s", task.Description))

	for i := 0; i < a.maxIterations; i++ {
		a.logger.WithFields(map[string]interface{}{
			"iteration": i + 1,
			"task_id":   task.TaskID,
		}).Info("agent iteration")

		action, err := a.callLLM()
		if err != nil {
			// If parse error, ask the LLM to correct itself and continue
			if strings.Contains(err.Error(), "failed to parse") {
				a.addMessage(model.RoleUser, "Your previous response was not valid JSON. Respond with ONLY a JSON object with an 'action' field.")
				continue
			}
			// Fatal LLM error
			a.logger.Error("LLM call failed: ", err)
			a.publishResult(task.TaskID, false, fmt.Sprintf("LLM error: %v", err), "")
			return
		}

		switch action.Action {
		case "execute":
			a.logger.WithFields(map[string]interface{}{
				"command": action.Command,
			}).Info("executing command")

			stdout, stderr, exitCode, err := RunCommand(context.Background(), action.Command, cmdTimeout)
			if err != nil {
				a.addMessage(model.RoleUser, fmt.Sprintf("Command execution error: %v", err))
				continue
			}

			result := formatCommandResult(stdout, stderr, exitCode)
			a.logger.WithFields(map[string]interface{}{
				"exit_code": exitCode,
			}).Info("command completed")
			a.addMessage(model.RoleUser, result)

		case "ask":
			a.logger.WithFields(map[string]interface{}{
				"question": action.Question,
			}).Info("asking user a question")

			response, err := a.askUser(task.TaskID, action.Question)
			if err != nil {
				a.logger.Warn("failed to get user response: ", err)
				a.addMessage(model.RoleUser, fmt.Sprintf("Failed to get user response: %v. Please proceed without this information or try a different approach.", err))
				continue
			}
			a.addMessage(model.RoleUser, fmt.Sprintf("User response: %s", response))

		case "complete":
			a.logger.WithFields(map[string]interface{}{
				"summary": action.Summary,
			}).Info("task completed successfully")
			a.publishResult(task.TaskID, true, action.Summary, action.Output)
			return

		case "error":
			a.logger.WithFields(map[string]interface{}{
				"summary": action.Summary,
			}).Error("task failed")
			a.publishResult(task.TaskID, false, action.Summary, "")
			return

		default:
			a.addMessage(model.RoleUser, fmt.Sprintf("Unknown action %q. Valid actions: execute, ask, complete, error.", action.Action))
		}
	}

	a.logger.Warn("max iterations reached")
	a.publishResult(task.TaskID, false, "max iterations reached without completing the task", "")
}

func (a *Agent) addMessage(role model.Role, content string) {
	a.messages = append(a.messages, model.Message{Role: role, Content: content})
}

func (a *Agent) callLLM() (*AgentAction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), llmTimeout)
	defer cancel()

	resp, err := a.provider.Complete(ctx, model.CompletionRequest{
		Model:       defaultModel,
		Messages:    a.messages,
		Temperature: 0.2,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// Add assistant response to history
	a.addMessage(model.RoleAssistant, resp.Content)

	action, err := parseAction(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w (raw: %s)", err, resp.Content)
	}

	return action, nil
}

func parseAction(content string) (*AgentAction, error) {
	cleaned := strings.TrimSpace(content)

	// Strip markdown code fences if present
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		if len(lines) >= 3 {
			cleaned = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	// Find JSON object boundaries
	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start >= 0 && end > start {
		cleaned = cleaned[start : end+1]
	}

	var action AgentAction
	if err := json.Unmarshal([]byte(cleaned), &action); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if action.Action == "" {
		return nil, fmt.Errorf("missing 'action' field in response")
	}

	return &action, nil
}

// askUser publishes a question on /llm.execute.question and waits for
// a response on /llm.execute.response via a temporary subscription.
func (a *Agent) askUser(taskID, question string) (string, error) {
	questionID := newQuestionID(taskID)

	// Dial a separate RX connection for the response subscription
	rxConn := topic.DialServer("localhost:11312")
	defer rxConn.Close()

	responseMsg := &msgs.ExecuteResponse{}
	topicType := fmt.Sprintf("%T", responseMsg)

	// Subscribe to the response topic
	env := msgs.Envelope{
		Cmd:       msgs.CmdSubscribe,
		Topic:     responseTopic,
		TopicType: topicType,
	}
	data, err := msgpack.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("marshal subscribe: %w", err)
	}
	if _, err := rxConn.Write(data); err != nil {
		return "", fmt.Errorf("send subscribe: %w", err)
	}

	// Publish the question
	a.node.Publish(questionTopic, &msgs.ExecuteQuestion{
		TaskID:     taskID,
		QuestionID: questionID,
		Question:   question,
	})

	// Wait for response with timeout
	if err := rxConn.SetReadDeadline(time.Now().Add(responseWait)); err != nil {
		return "", fmt.Errorf("set read deadline: %w", err)
	}

	for {
		var respEnv msgs.Envelope
		if err := msgpack.UnmarshalRead(rxConn, &respEnv); err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return "", fmt.Errorf("timed out waiting for user response")
			}
			return "", fmt.Errorf("read response: %w", err)
		}

		switch respEnv.Cmd {
		case msgs.RespOK:
			continue // subscribe acknowledgment
		case msgs.RespError:
			return "", fmt.Errorf("server error: %s", respEnv.Err)
		case msgs.RespMessage:
			// decode the payload
		default:
			continue
		}

		if err := msgpack.Unmarshal(respEnv.Payload, responseMsg); err != nil {
			return "", fmt.Errorf("unmarshal response: %w", err)
		}

		if !matchesResponse(taskID, questionID, responseMsg) {
			continue
		}

		return responseMsg.Response, nil
	}
}

func matchesResponse(taskID, questionID string, response *msgs.ExecuteResponse) bool {
	if response == nil {
		return false
	}
	if response.QuestionID != "" {
		return response.QuestionID == questionID
	}
	if taskID == "" {
		return response.TaskID == ""
	}
	return response.TaskID == taskID
}

func newQuestionID(taskID string) string {
	if taskID == "" {
		return fmt.Sprintf("question-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d", taskID, time.Now().UnixNano())
}

func (a *Agent) publishResult(taskID string, success bool, summary, output string) {
	a.node.Publish(resultTopic, &msgs.ExecuteResult{
		TaskID:  taskID,
		Success: success,
		Summary: summary,
		Output:  output,
	})
}

func formatCommandResult(stdout, stderr string, exitCode int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Exit code: %d\n", exitCode))
	if stdout != "" {
		sb.WriteString(fmt.Sprintf("Stdout:\n%s\n", stdout))
	}
	if stderr != "" {
		sb.WriteString(fmt.Sprintf("Stderr:\n%s\n", stderr))
	}
	if stdout == "" && stderr == "" {
		sb.WriteString("(no output)\n")
	}
	return sb.String()
}

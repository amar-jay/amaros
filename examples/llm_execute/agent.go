package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/amar-jay/amaros/internal/config"
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

2. Ask the user a question (this is shorthand for a topic request over /llm.execute.question -> /llm.execute.response, and only works when /llm.execute.question is currently publishable):
{"action": "ask", "question": "<your question>"}

3. Publish structured data to an available topic:
{"action": "topic_publish", "topic": "<topic name>", "payload": {"key": "value"}}

4. Publish to one topic and wait for a correlated reply on another topic:
{"action": "topic_request", "request_topic": "<topic name>", "payload": {"key": "value"}, "response_topic": "<topic name>", "match_field": "<response field name>", "match_value": "<expected field value>", "timeout_seconds": 120}

5. Report successful task completion:
{"action": "complete", "summary": "<brief summary of what was done>", "output": "<relevant output or result>"}

6. Report failure:
{"action": "error", "summary": "<what went wrong>"}

Topic Usage Rules:
[topic_usage_rules]

Available Topics:
[topics_list]

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
	Topic    string `json:"topic,omitempty"`
	RequestTopic string `json:"request_topic,omitempty"`
	ResponseTopic string `json:"response_topic,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	MatchField string `json:"match_field,omitempty"`
	MatchValue string `json:"match_value,omitempty"`
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Output   string `json:"output,omitempty"`
}

// Agent runs the agentic loop for task execution.
type Agent struct {
	provider      model.Provider
	node          *node.Node
	maxIterations int
	logger        *ilog.Logger
	topicCatalog  []promptTopic
	messages      []model.Message
}

type promptTopic struct {
	Name      string
	Type      string
	Publishable bool
	Waitable  bool
	Subscribers int
	OwnerNode string
	Purpose   string
	RequestTopic string
	ResponseTopic string
	ResponseType string
	Source    string
}

// NewAgent creates a new Agent.
func NewAgent(p model.Provider, n *node.Node, t []topic.Topic, maxIter int) *Agent {
	a := &Agent{
		provider:      p,
		node:          n,
		maxIterations: maxIter,
		logger:        ilog.New(),
		topicCatalog:  buildPromptTopics(t),
	}
	a.setSystemPrompt()
	return a
}

func buildPromptTopics(observedTopics []topic.Topic) []promptTopic {
	runtimeTopics := make(map[string]topic.Topic, len(observedTopics))

	for _, observedTopic := range observedTopics {
		if observedTopic.Name == "" {
			continue
		}
		existing := runtimeTopics[observedTopic.Name]
		if existing.Type == "" {
			existing.Type = observedTopic.Type
		}
		if observedTopic.Type != "" {
			existing.Type = observedTopic.Type
		}
		if observedTopic.Subscribers > existing.Subscribers {
			existing.Subscribers = observedTopic.Subscribers
		}
		existing.Name = observedTopic.Name
		runtimeTopics[observedTopic.Name] = existing
	}

	merged := make(map[string]promptTopic, len(runtimeTopics))

	for _, observedTopic := range runtimeTopics {
		entry := promptTopic{
			Name:        observedTopic.Name,
			Type:        observedTopic.Type,
			Publishable: observedTopic.Subscribers > 0,
			Waitable:    true,
			Subscribers: observedTopic.Subscribers,
			OwnerNode:   observedTopic.OwnerNode,
			Purpose:     observedTopic.Purpose,
			RequestTopic: observedTopic.RequestTopic,
			ResponseTopic: observedTopic.ResponseTopic,
			ResponseType: observedTopic.ResponseType,
			Source:      "runtime",
		}

		if existing, ok := merged[observedTopic.Name]; ok {
			merged[observedTopic.Name] = mergePromptTopic(existing, entry)
			continue
		}

		merged[observedTopic.Name] = entry
	}

	result := make([]promptTopic, 0, len(merged))
	for _, entry := range merged {
		if entry.Type == "" {
			entry.Type = "unknown"
		}
		if entry.Source == "" {
			entry.Source = "runtime"
		}
		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].Source < result[j].Source
		}
		return result[i].Name < result[j].Name
	})

	return result
}

func mergePromptTopic(current, incoming promptTopic) promptTopic {
	merged := current
	if merged.Type == "" || merged.Type == "unknown" {
		merged.Type = incoming.Type
	}
	merged.Publishable = merged.Publishable || incoming.Publishable
	merged.Waitable = merged.Waitable || incoming.Waitable
	if incoming.Subscribers > merged.Subscribers {
		merged.Subscribers = incoming.Subscribers
	}
	if merged.OwnerNode == "" {
		merged.OwnerNode = incoming.OwnerNode
	}
	if merged.Purpose == "" {
		merged.Purpose = incoming.Purpose
	}
	if merged.RequestTopic == "" {
		merged.RequestTopic = incoming.RequestTopic
	}
	if merged.ResponseTopic == "" {
		merged.ResponseTopic = incoming.ResponseTopic
	}
	if merged.ResponseType == "" {
		merged.ResponseType = incoming.ResponseType
	}
	if merged.Source == "" {
		merged.Source = incoming.Source
	}
	return merged
}

func formatAvailableTopics(topics []promptTopic) string {
	if len(topics) == 0 {
		return "- No topics were discoverable at task start. Use /llm.execute.question when you need user input and expect answers on /llm.execute.response."
	}

	var builder strings.Builder
	for _, availableTopic := range topics {
		topicType := availableTopic.Type
		if topicType == "" {
			topicType = "unknown"
		}

		builder.WriteString("- ")
		builder.WriteString(availableTopic.Name)
		builder.WriteString(" | type: ")
		builder.WriteString(topicType)
		builder.WriteString(" | publishable: ")
		builder.WriteString(strconv.FormatBool(availableTopic.Publishable))
		builder.WriteString(" | waitable: ")
		builder.WriteString(strconv.FormatBool(availableTopic.Waitable))
		builder.WriteString(" | subscribers: ")
		builder.WriteString(strconv.Itoa(availableTopic.Subscribers))

		if availableTopic.OwnerNode != "" {
			builder.WriteString(" | owner: ")
			builder.WriteString(availableTopic.OwnerNode)
		}

		if availableTopic.Purpose != "" {
			builder.WriteString(" | purpose: ")
			builder.WriteString(availableTopic.Purpose)
		}

		if availableTopic.RequestTopic != "" {
			builder.WriteString(" | request_topic: ")
			builder.WriteString(availableTopic.RequestTopic)
		}

		if availableTopic.ResponseTopic != "" {
			builder.WriteString(" | response_topic: ")
			builder.WriteString(availableTopic.ResponseTopic)
		}

		if availableTopic.ResponseType != "" {
			builder.WriteString(" | response_type: ")
			builder.WriteString(availableTopic.ResponseType)
		}

		if availableTopic.Source != "" {
			builder.WriteString(" | source: ")
			builder.WriteString(availableTopic.Source)
		}

		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

func topicUsageRules(topics []promptTopic) string {
	var builder strings.Builder
	builder.WriteString("- Treat topics as shared coordination channels. Only publish to topics whose publishable flag is true in the current topic list.\n")
	builder.WriteString("- Topic purpose and request-response routing come from the node that owns the topic. Prefer those advertised semantics over guessing from topic names.\n")
	builder.WriteString("- Use the ask action only when /llm.execute.question is publishable=true and the topic advertises a response_topic.\n")
	builder.WriteString("- Use topic_publish to send structured payloads to any currently publishable topic.\n")
	builder.WriteString("- Use topic_request when you need a request-response flow: publish to request_topic, then wait on response_topic. If response_topic is omitted, use the response_topic advertised by the request topic.\n")
	builder.WriteString("- A response topic may be waitable even when publishable=false. That means the executor can subscribe and wait on it, but should not publish to it.\n")
	builder.WriteString("- Do not try to publish to /llm.execute.response yourself. That topic is for answers coming back into the executor.\n")
	builder.WriteString("- Do not manually publish to /llm.execute.result during reasoning. The executor publishes the final result automatically when you return the complete or error action.\n")
	builder.WriteString("- If a topic type is unknown, inspect it conservatively before relying on it.\n")

	if len(topics) > 0 {
		builder.WriteString("- The topic list below is deduplicated by topic name and enriched with owner-provided metadata when available.\n")
	}

	return strings.TrimSpace(builder.String())
}

// Run executes the agentic loop for the given task.
func (a *Agent) Run(task *msgs.ExecuteTask) {
	a.logger.WithFields(map[string]interface{}{
		"task_id":     task.TaskID,
		"description": task.Description,
	}).Info("starting agentic loop")

	a.addMessage(model.RoleUser, fmt.Sprintf("Task: %s", task.Description))

	for i := 0; i < a.maxIterations; i++ {
		if err := a.refreshTopicCatalog(); err != nil {
			a.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("failed to refresh runtime topic catalog")
		}

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

		case "topic_publish":
			result, err := a.publishToTopic(action)
			if err != nil {
				a.addMessage(model.RoleUser, fmt.Sprintf("Topic publish error: %v", err))
				continue
			}
			a.addMessage(model.RoleUser, result)

		case "topic_request":
			result, err := a.requestTopic(action)
			if err != nil {
				a.addMessage(model.RoleUser, fmt.Sprintf("Topic request error: %v", err))
				continue
			}
			a.addMessage(model.RoleUser, result)

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

func (a *Agent) setSystemPrompt() {
	content := strings.ReplaceAll(systemPrompt, "[topics_list]", formatAvailableTopics(a.topicCatalog))
	content = strings.ReplaceAll(content, "[topic_usage_rules]", topicUsageRules(a.topicCatalog))
	if len(a.messages) == 0 {
		a.messages = []model.Message{{Role: model.RoleSystem, Content: content}}
		return
	}
	a.messages[0] = model.Message{Role: model.RoleSystem, Content: content}
}

func (a *Agent) refreshTopicCatalog() error {
	runtimeTopics, err := fetchRuntimeTopics()
	if err != nil {
		return err
	}
	a.topicCatalog = buildPromptTopics(runtimeTopics)
	a.setSystemPrompt()
	return nil
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

func fetchRuntimeTopics() ([]topic.Topic, error) {
	conf := config.Get()
	url := net.JoinHostPort(conf.Core.Host, strconv.Itoa(conf.Core.RxPort))
	conn, err := net.Dial("tcp", url)
	if err != nil {
		return nil, fmt.Errorf("dial topic server: %w", err)
	}
	defer conn.Close()

	topics, err := topic.FetchList(conn)
	if err != nil {
		return nil, fmt.Errorf("fetch topic list: %w", err)
	}
	return topics, nil
}

func (a *Agent) publishToTopic(action *AgentAction) (string, error) {
	if action.Topic == "" {
		return "", fmt.Errorf("missing topic field")
	}

	entry, ok := a.lookupTopic(action.Topic)
	if !ok || !entry.Publishable {
		return "", fmt.Errorf("topic %s is not currently publishable", action.Topic)
	}

	payload, err := decodeActionPayload(action.Payload)
	if err != nil {
		return "", err
	}

	a.node.Publish(action.Topic, payload)
	return fmt.Sprintf("Published payload to %s: %s", action.Topic, formatPayloadForPrompt(payload)), nil
}

func (a *Agent) requestTopic(action *AgentAction) (string, error) {
	if action.RequestTopic == "" {
		return "", fmt.Errorf("missing request_topic field")
	}
	if action.ResponseTopic == "" {
		return "", fmt.Errorf("missing response_topic field")
	}

	requestEntry, ok := a.lookupTopic(action.RequestTopic)
	if !ok || !requestEntry.Publishable {
		return "", fmt.Errorf("request topic %s is not currently publishable", action.RequestTopic)
	}

	responseTopicName := action.ResponseTopic
	responseTopicType := ""
	if responseTopicName == "" {
		responseTopicName = requestEntry.ResponseTopic
		responseTopicType = requestEntry.ResponseType
	}
	if responseTopicName == "" {
		return "", fmt.Errorf("request topic %s does not advertise a response topic and response_topic was not provided", action.RequestTopic)
	}

	responseEntry, ok := a.lookupTopic(responseTopicName)
	if !ok || !responseEntry.Waitable {
		if responseTopicType == "" {
			return "", fmt.Errorf("response topic %s is not available as a waitable topic", responseTopicName)
		}
	}
	if responseTopicType == "" {
		responseTopicType = responseEntry.ResponseType
	}
	if responseTopicType == "" {
		responseTopicType = responseEntry.Type
	}
	if responseTopicType == "" || responseTopicType == "unknown" {
		return "", fmt.Errorf("response topic %s has unknown type", responseTopicName)
	}

	payload, err := decodeActionPayload(action.Payload)
	if err != nil {
		return "", err
	}

	timeout := responseWait
	if action.TimeoutSeconds > 0 {
		timeout = time.Duration(action.TimeoutSeconds) * time.Second
	}

	responsePayload, err := a.publishAndWait(action.RequestTopic, payload, responseTopicName, responseTopicType, action.MatchField, action.MatchValue, timeout)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Received response from %s: %s", responseTopicName, formatPayloadForPrompt(responsePayload)), nil
}

// askUser publishes a question on /llm.execute.question and waits for
// a response on /llm.execute.response via a temporary subscription.
func (a *Agent) askUser(taskID, question string) (string, error) {
	questionEntry, ok := a.lookupTopic(questionTopic)
	if !ok || !questionEntry.Publishable {
		return "", fmt.Errorf("%s is not currently publishable; ask action is unavailable", questionTopic)
	}
	if questionEntry.ResponseTopic == "" {
		return "", fmt.Errorf("%s does not advertise a response_topic; ask action is unavailable", questionTopic)
	}

	questionID := newQuestionID(taskID)
	conf := config.Get()

	// Dial a separate RX connection for the response subscription
	url := net.JoinHostPort(conf.Core.Host, strconv.Itoa(conf.Core.RxPort))
	rxConn := topic.DialServer(url)
	defer rxConn.Close()

	responseMsg := &msgs.ExecuteResponse{}
	topicType := firstNonEmpty(questionEntry.ResponseType, fmt.Sprintf("%T", responseMsg))

	// Subscribe to the response topic
	env := msgs.Envelope{
		Cmd:       msgs.CmdSubscribe,
		Topic:     questionEntry.ResponseTopic,
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

func (a *Agent) publishAndWait(requestTopic string, payload interface{}, responseTopicName string, responseTopicType string, matchField string, matchValue string, timeout time.Duration) (interface{}, error) {
	conf := config.Get()
	url := net.JoinHostPort(conf.Core.Host, strconv.Itoa(conf.Core.RxPort))
	rxConn, err := net.Dial("tcp", url)
	if err != nil {
		return nil, fmt.Errorf("dial response topic server: %w", err)
	}
	defer rxConn.Close()

	env := msgs.Envelope{
		Cmd:       msgs.CmdSubscribe,
		Topic:     responseTopicName,
		TopicType: responseTopicType,
	}
	data, err := msgpack.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal subscribe: %w", err)
	}
	if _, err := rxConn.Write(data); err != nil {
		return nil, fmt.Errorf("send subscribe: %w", err)
	}

	a.node.Publish(requestTopic, payload)

	if err := rxConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}

	for {
		var respEnv msgs.Envelope
		if err := msgpack.UnmarshalRead(rxConn, &respEnv); err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return nil, fmt.Errorf("timed out waiting for response on %s", responseTopicName)
			}
			return nil, fmt.Errorf("read response: %w", err)
		}

		switch respEnv.Cmd {
		case msgs.RespOK:
			continue
		case msgs.RespError:
			return nil, fmt.Errorf("server error: %s", respEnv.Err)
		case msgs.RespMessage:
		default:
			continue
		}

		var responsePayload interface{}
		if err := msgpack.Unmarshal(respEnv.Payload, &responsePayload); err != nil {
			return nil, fmt.Errorf("unmarshal response payload: %w", err)
		}

		if !matchesGenericResponse(responsePayload, matchField, matchValue) {
			continue
		}

		return responsePayload, nil
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

func matchesGenericResponse(responsePayload interface{}, matchField, matchValue string) bool {
	if matchField == "" {
		return true
	}

	responseMap, ok := responsePayload.(map[string]interface{})
	if !ok {
		return false
	}

	actualValue, ok := responseMap[matchField]
	if !ok {
		return false
	}

	return fmt.Sprint(actualValue) == matchValue
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

func (a *Agent) lookupTopic(topicName string) (promptTopic, bool) {
	for _, availableTopic := range a.topicCatalog {
		if availableTopic.Name == topicName {
			return availableTopic, true
		}
	}
	return promptTopic{}, false
}

func decodeActionPayload(raw json.RawMessage) (interface{}, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("missing payload field")
	}

	var payload interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}
	return payload, nil
}

func formatPayloadForPrompt(payload interface{}) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("%v", payload)
	}
	return string(data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type questionMsg msgs.ExecuteQuestion

type resultMsg msgs.ExecuteResult

type model struct {
	node          *node.Node
	requestTopic  string
	responseTopic string
	taskTopic     string
	resultTopic   string

	input      textinput.Model
	current    *msgs.ExecuteQuestion
	createTask bool
	history    []string

	questionCh chan msgs.ExecuteQuestion
	resultCh   chan msgs.ExecuteResult
	err        error
}

func main() {
	nodeName := flag.String("node-name", "console_messaging", "name of this node")
	requestTopic := flag.String("request-topic", "/console.question", "topic to subscribe for incoming questions")
	responseTopic := flag.String("response-topic", "/console.response", "topic to publish answers")
	taskTopic := flag.String("task-topic", "/llm.execute.task", "topic to send tasks to the llm_execute node")
	resultTopic := flag.String("result-topic", "/llm.execute.result", "topic to subscribe for execution results")
	flag.Parse()

	// Initialize the node and reveal what we publish so others can discover it.
	n := node.Init(node.NodeConfig{Name: *nodeName})
	n.DescribeTopics([]msgs.TopicMetadata{
		{
			Topic:         *requestTopic,
			Type:          msgs.GetType(msgs.ExecuteQuestion{}),
			Purpose:       "questions that require a human answer through the llm_question_answer node",
			ResponseTopic: *responseTopic,
			ResponseType:  msgs.GetType(msgs.ExecuteResponse{}),
		},
		{
			Topic:   *responseTopic,
			Type:    msgs.GetType(msgs.ExecuteResponse{}),
			Purpose: "answers returned by the llm_question_answer node to previously asked questions",
		},
		{
			Topic:   *taskTopic,
			Type:    msgs.GetType(msgs.ExecuteTask{}),
			Purpose: "tasks submitted by console_messaging to be executed by the llm_execute node",
		},
	})

	// Setup clean shutdown on Ctrl+C.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\nshutting down console_messaging node")
		os.Exit(0)
	}()

	// Channels used to forward messages into the TUI.
	questionCh := make(chan msgs.ExecuteQuestion, 16)
	resultCh := make(chan msgs.ExecuteResult, 16)

	// Subscribe to incoming questions.
	question := &msgs.ExecuteQuestion{}
	go func() {
		n.SubscribeWithCallback(*requestTopic, question, func(ctx topic.CallbackContext) {
			req := *question // copy incoming message before it is reused.
			if req.Question == "" {
				ctx.Logger.Warn("received empty question, skipping")
				return
			}
			questionCh <- req
		})
	}()

	// Subscribe to execution results.
	result := &msgs.ExecuteResult{}
	go func() {
		n.SubscribeWithCallback(*resultTopic, result, func(_ topic.CallbackContext) {
			res := *result
			resultCh <- res
		})
	}()

	m := newModel(n, *requestTopic, *responseTopic, *taskTopic, *resultTopic, questionCh, resultCh)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start UI: %v\n", err)
		os.Exit(1)
	}
}

func newModel(n *node.Node, requestTopic, responseTopic, taskTopic, resultTopic string, questionCh chan msgs.ExecuteQuestion, resultCh chan msgs.ExecuteResult) model {
	i := textinput.New()
	i.Placeholder = "Type your answer and press Enter"
	i.Focus()
	i.CharLimit = 512
	i.Width = 60

	return model{
		node:          n,
		requestTopic:  requestTopic,
		responseTopic: responseTopic,
		taskTopic:     taskTopic,
		resultTopic:   resultTopic,
		input:         i,
		questionCh:    questionCh,
		resultCh:      resultCh,
		history:       make([]string, 0, 16),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(waitForQuestion(m.questionCh), waitForResult(m.resultCh))
}

func waitForQuestion(ch chan msgs.ExecuteQuestion) tea.Cmd {
	return func() tea.Msg {
		q := <-ch
		return questionMsg(q)
	}
}

func waitForResult(ch chan msgs.ExecuteResult) tea.Cmd {
	return func() tea.Msg {
		r := <-ch
		return resultMsg(r)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case questionMsg:
		r := msgs.ExecuteQuestion(msg)
		m.current = &r
		m.input.SetValue("")
		m.input.Placeholder = "Type your answer and press Enter"
		return m, nil

	case resultMsg:
		res := msgs.ExecuteResult(msg)
		m.history = append([]string{formatResult(res)}, m.history...)
		if len(m.history) > 20 {
			m.history = m.history[:20]
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.createTask {
				m.createTask = false
				m.input.SetValue("")
				m.input.Placeholder = "Type your answer and press Enter"
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+t":
			if m.current == nil {
				m.createTask = true
				m.input.SetValue("")
				m.input.Placeholder = "Type a task description and press Enter"
			}
			return m, nil
		case "enter":
			if m.current != nil {
				answer := strings.TrimSpace(m.input.Value())
				if answer != "" {
					resp := msgs.ExecuteResponse{
						TaskID:     m.current.TaskID,
						QuestionID: m.current.QuestionID,
						Response:   answer,
					}
					m.node.Publish(m.responseTopic, resp)
					m.history = append([]string{fmt.Sprintf("Q: %s\nA: %s", m.current.Question, answer)}, m.history...)
					m.current = nil
					m.input.SetValue("")
				}
			} else if m.createTask {
				desc := strings.TrimSpace(m.input.Value())
				if desc != "" {
					task := msgs.ExecuteTask{Description: desc}
					m.node.Publish(m.taskTopic, task)
					m.history = append([]string{fmt.Sprintf("Task sent: %s", desc)}, m.history...)
					m.createTask = false
					m.input.SetValue("")
					m.input.Placeholder = "Type your answer and press Enter"
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("AMAROS Console Messaging\n")
	b.WriteString(fmt.Sprintf("subscribe: %s   publish: %s   results: %s\n", m.requestTopic, m.responseTopic, m.resultTopic))
	b.WriteString("────────────────────────────────────────────────────────────────\n")

	if m.current != nil {
		b.WriteString(fmt.Sprintf("Question: %s\n\n", m.current.Question))
		b.WriteString(m.input.View())
		b.WriteString("\n")
	} else if m.createTask {
		b.WriteString("Create task (press Esc to cancel):\n")
		b.WriteString(m.input.View())
		b.WriteString("\n")
	} else {
		b.WriteString("Waiting for question... (Ctrl+C to quit)\n")
		b.WriteString("Press 'ctrl+t' to submit a task to llm_execute\n")
	}

	if len(m.history) > 0 {
		b.WriteString("\nRecent activity:\n")
		for i, entry := range m.history {
			if i >= 5 {
				break
			}
			b.WriteString(entry + "\n")
			b.WriteString("────────────────────────────────────────\n")
		}
	}

	if m.err != nil {
		b.WriteString("\nERROR: " + m.err.Error() + "\n")
	}

	return b.String()
}

func formatResult(r msgs.ExecuteResult) string {
	status := "FAILED"
	if r.Success {
		status = "SUCCESS"
	}
	out := fmt.Sprintf("Result [%s] %s: %s", r.TaskID, status, r.Summary)
	if r.Output != "" {
		out += "\n" + strings.TrimSpace(r.Output)
	}
	return out
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mitchellh/mapstructure"

	"github.com/amar-jay/amaros/pkg/config"
	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
)

// task: can ask and recieve response via the the question and response topics. and
// tasks can be sent via a topic
const (
	requestTopic   = "/telegram.question"
	responseTopic  = "/telegram.response"
	requestTimeout = 60 * time.Second
)

var (
	llmNode      *node.Node
	question     = &msgs.ExecuteQuestion{}
	telegram     *TelegramClient
	result       = &msgs.ExecuteResult{}
	taskDispatch = make(map[string]taskSubscription)
	taskMu       sync.Mutex
)

type taskSubscription struct {
	chatID      int64
	description string
}

type TelegramClient struct {
	client    *Tclient
	ctx       context.Context
	cancel    context.CancelFunc
	chatIDs   map[int64]bool
	chatIDsMu sync.RWMutex
	pending   map[string]chan string
	pendingMu sync.Mutex
}

func newTelegramClient(botToken string) (*TelegramClient, error) {
	client, err := newClient(Tconfig{BotToken: botToken})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	tc := &TelegramClient{
		client:  client,
		ctx:     ctx,
		cancel:  cancel,
		chatIDs: make(map[int64]bool),
		pending: make(map[string]chan string),
	}

	updates, err := client.startPolling()
	if err != nil {
		cancel()
		return nil, err
	}

	go tc.handleUpdates(updates)

	return tc, nil
}

func (tc *TelegramClient) handleUpdates(updates tgbotapi.UpdatesChannel) {
	for {
		select {
		case <-tc.ctx.Done():
			return
		case update := <-updates:
			if update.Message != nil {
				chatID := update.Message.Chat.ID
				tc.chatIDsMu.Lock()
				tc.chatIDs[chatID] = true
				tc.chatIDsMu.Unlock()

				text := update.Message.Text
				if text != "" {
					// If the message is a command, handle it and do not treat it as an answer.
					if tc.handleCommand(chatID, text) {
						continue
					}

					tc.pendingMu.Lock()
					for questionID, ch := range tc.pending {
						select {
						case ch <- text:
							delete(tc.pending, questionID)
						default:
						}
					}
					tc.pendingMu.Unlock()
				}
			}
		}
	}
}

func (tc *TelegramClient) handleCommand(chatID int64, text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return false
	}

	cmd := fields[0]
	switch cmd {
	case "/llm.execute.task":
		desc := strings.TrimSpace(strings.TrimPrefix(text, cmd))
		if desc == "" {
			_, _ = tc.client.sendMessage(chatID, "Usage: /llm.execute.task <task description>")
			return true
		}

		taskID := fmt.Sprintf("telegram-%d-%d", chatID, time.Now().UnixNano())
		task := msgs.ExecuteTask{
			TaskID:      taskID,
			Description: desc,
		}

		taskMu.Lock()
		taskDispatch[taskID] = taskSubscription{chatID: chatID, description: desc}
		taskMu.Unlock()

		llmNode.Publish("/llm.execute.task", task)
		_, _ = tc.client.sendMessage(chatID, "Published task to /llm.execute.task")
		return true
	default:
		return false
	}
}

func (tc *TelegramClient) askQuestion(questionID, questionText string) (string, error) {
	tc.chatIDsMu.RLock()
	var chatIDs []int64
	for chatID := range tc.chatIDs {
		chatIDs = append(chatIDs, chatID)
	}
	tc.chatIDsMu.RUnlock()

	if len(chatIDs) == 0 && tc.client.config.ChatId == "" {
		return "", fmt.Errorf("no users have messaged the bot yet and no chat_id is configured. Message the bot first to register or set integrations.telegram.chat_id in your config.")
	}

	answerCh := make(chan string, 1)
	tc.pendingMu.Lock()
	tc.pending[questionID] = answerCh
	tc.pendingMu.Unlock()

	defer func() {
		tc.pendingMu.Lock()
		delete(tc.pending, questionID)
		tc.pendingMu.Unlock()
	}()

	// Notify registered chat IDs first.
	for _, chatID := range chatIDs {
		_, _ = tc.client.sendMessage(chatID, questionText)
	}

	// If no users have messaged yet, use the configured chat_id as a fallback.
	if len(chatIDs) == 0 && tc.client.config.ChatId != "" {
		// Allow numeric chat IDs or channel/user handles (e.g. @username).
		if id, err := strconv.ParseInt(tc.client.config.ChatId, 10, 64); err == nil {
			_, _ = tc.client.sendMessage(id, questionText)
		} else {
			_, _ = tc.client.sendMessageToChannel(tc.client.config.ChatId, questionText)
		}
	}

	select {
	case answer := <-answerCh:
		return strings.TrimSpace(answer), nil
	case <-time.After(requestTimeout):
		return "", fmt.Errorf("timeout waiting for answer")
	}
}

func (tc *TelegramClient) close() {
	tc.cancel()
	tc.client.stopPolling()
}

func init() {
	c := config.Get()

	raw, ok := c.Integrations["telegram"]
	if !ok {
		log.Fatal("telegram integration config missing")
	}

	var t Tconfig
	if err := mapstructure.Decode(raw, &t); err != nil {
		log.Fatalf("failed to decode telegram integration config: %v", err)
	}

	if t.BotToken == "" {
		log.Fatal("telegram bot_token is required in config")
	}

	var err error
	telegram, err = newTelegramClient(t.BotToken)
	if err != nil {
		log.Fatalf("failed to initialize telegram client: %v", err)
	}

	llmNode = node.Init("telegram_messaging")
	llmNode.DescribeTopics([]msgs.TopicMetadata{
		{
			Topic:         requestTopic,
			Type:          msgs.GetType(msgs.ExecuteQuestion{}),
			Purpose:       "questions that require a human answer through the llm_question_answer node",
			ResponseTopic: responseTopic,
			ResponseType:  msgs.GetType(msgs.ExecuteResponse{}),
		},
		{
			Topic:   responseTopic,
			Type:    msgs.GetType(msgs.ExecuteResponse{}),
			Purpose: "answers returned by the llm_question_answer node to previously asked questions",
		},
		{
			Topic:   "/llm.execute.result",
			Type:    msgs.GetType(msgs.ExecuteResult{}),
			Purpose: "task results sent back to the requester",
		},
	})
	llmNode.OnShutdown(func() {
		fmt.Println("shutting down telegram_messaging node")
		telegram.close()
	})
}

func onRequest(ctx topic.CallbackContext) {
	req := *question
	if req.Question == "" {
		ctx.Logger.Warn("received empty question, skipping")
		return
	}

	ctx.Logger.WithFields(map[string]interface{}{
		"task_id":     req.TaskID,
		"question_id": req.QuestionID,
		"question":    req.Question,
	}).Info("Received question")

	answer, err := telegram.askQuestion(req.QuestionID, req.Question)
	if err != nil {
		ctx.Logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("failed to get answer")
		return
	}

	ctx.Logger.Info("Received answer")

	response := msgs.ExecuteResponse{
		TaskID:     req.TaskID,
		QuestionID: req.QuestionID,
		Response:   answer,
	}

	llmNode.Publish(responseTopic, response)
}

func main() {
	// Ensure log output is visible even when the process blocks.
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("telegram_messaging node started")
	log.Printf("  subscribed to: %s", requestTopic)
	log.Printf("  publishing to: %s", responseTopic)

	// Run subscriptions concurrently so we can handle multiple topics.
	llmNode.Callback(onRequest)
	llmNode.Subscribe(requestTopic, question)
}

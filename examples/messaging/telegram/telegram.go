package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Tconfig struct {
	BotToken string `yaml:"bot_token" mapstructure:"bot_token"`
	ChatId   string `yaml:"chat_id" mapstructure:"chat_id"`
	//WebhookURL string
	//UseWebhook bool
}

type Tclient struct {
	bot           *tgbotapi.BotAPI
	config        Tconfig
	updateChannel tgbotapi.UpdatesChannel
	httpClient    *http.Client
}

func newClient(cfg Tconfig) (*Tclient, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	bot.Debug = false

	return &Tclient{
		bot:        bot,
		config:     cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Tclient) botToken() string {
	return c.bot.Token
}

func (c *Tclient) sendMessage(chatID int64, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = false
	return c.bot.Send(msg)
}

func (c *Tclient) sendMessageToChannel(channel string, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessageToChannel(channel, text)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = false
	return c.bot.Send(msg)
}

func (c *Tclient) sendMessageWithKeyboard(chatID int64, text string, keyboard interface{}) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	return c.bot.Send(msg)
}

func (c *Tclient) sendPhoto(chatID int64, photo string, caption string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(photo))
	if caption != "" {
		msg.Caption = caption
	}
	msg.ParseMode = "Markdown"
	return c.bot.Send(msg)
}

func (c *Tclient) sendDocument(chatID int64, document string, caption string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewDocument(chatID, tgbotapi.FileURL(document))
	if caption != "" {
		msg.Caption = caption
	}
	msg.ParseMode = "Markdown"
	return c.bot.Send(msg)
}

func (c *Tclient) editMessageText(chatID int64, messageID int, text string) error {
	msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	msg.ParseMode = "Markdown"
	_, err := c.bot.Send(msg)
	return err
}

func (c *Tclient) deleteMessage(chatID int64, messageID int) error {
	msg := tgbotapi.NewDeleteMessage(chatID, messageID)
	_, err := c.bot.Send(msg)
	return err
}

func (c *Tclient) answerCallbackQuery(callbackQueryID string, text string) error {
	callback := tgbotapi.NewCallback(callbackQueryID, text)
	_, err := c.bot.Request(callback)
	return err
}

func (c *Tclient) getChat(chatID int64) (tgbotapi.Chat, error) {
	return c.bot.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: chatID}})
}

func (c *Tclient) getChatAdministrators(chatID int64) ([]tgbotapi.ChatMember, error) {
	return c.bot.GetChatAdministrators(tgbotapi.ChatAdministratorsConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: chatID}})
}

func (c *Tclient) getChatMember(chatID int64, userID int64) (tgbotapi.ChatMember, error) {
	return c.bot.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{ChatID: chatID, UserID: userID},
	})
}

func (c *Tclient) leaveChat(chatID int64) error {
	params := make(tgbotapi.Params)
	params.AddNonZero64("chat_id", chatID)
	_, err := c.bot.MakeRequest("leaveChat", params)
	return err
}

func (c *Tclient) setWebhook(url string) error {
	cfg, err := tgbotapi.NewWebhook(url)
	if err != nil {
		return fmt.Errorf("failed to create webhook config: %w", err)
	}
	_, err = c.bot.Request(cfg)
	return err
}

func (c *Tclient) removeWebhook() error {
	_, err := c.bot.Request(tgbotapi.DeleteWebhookConfig{})
	return err
}

func (c *Tclient) getWebhookInfo() (tgbotapi.WebhookInfo, error) {
	return c.bot.GetWebhookInfo()
}

func (c *Tclient) startPolling() (tgbotapi.UpdatesChannel, error) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	return c.bot.GetUpdatesChan(u), nil
}

func (c *Tclient) stopPolling() {
	c.bot.StopReceivingUpdates()
}

func (c *Tclient) handleUpdates(ctx context.Context, handler func(tgbotapi.Update)) {
	if c.updateChannel == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			c.stopPolling()
			return
		case update := <-c.updateChannel:
			if update.UpdateID != 0 {
				handler(update)
			}
		}
	}
}

func newInlineKeyboardMarkup(buttons [][]inlineKeyboardButton) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, len(buttons))
	for i, row := range buttons {
		rows[i] = make([]tgbotapi.InlineKeyboardButton, len(row))
		for j, btn := range row {
			rows[i][j] = tgbotapi.InlineKeyboardButton{
				Text:         btn.Text,
				URL:          &btn.URL,
				CallbackData: &btn.CallbackData,
			}
		}
	}
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func newReplyKeyboardMarkup(buttons [][]keyboardButton) tgbotapi.ReplyKeyboardMarkup {
	rows := make([][]tgbotapi.KeyboardButton, len(buttons))
	for i, row := range buttons {
		rows[i] = make([]tgbotapi.KeyboardButton, len(row))
		for j, btn := range row {
			rows[i][j] = tgbotapi.KeyboardButton{
				Text:            btn.Text,
				RequestContact:  btn.RequestContact,
				RequestLocation: btn.RequestLocation,
			}
		}
	}
	return tgbotapi.ReplyKeyboardMarkup{Keyboard: rows, ResizeKeyboard: true}
}

type inlineKeyboardButton struct {
	Text         string
	URL          string
	CallbackData string
}

type keyboardButton struct {
	Text            string
	RequestContact  bool
	RequestLocation bool
}

func (c *Tclient) getMe() (tgbotapi.User, error) {
	return c.bot.GetMe()
}

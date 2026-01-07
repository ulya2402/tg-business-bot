package api

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type TelegramClient struct {
	Token   string
	BaseURL string
}

type Update struct {
	UpdateID           int64               `json:"update_id"`
	Message            *Message            `json:"message"`
	BusinessConnection *BusinessConnection `json:"business_connection"`
	BusinessMessage    *Message            `json:"business_message"`
	CallbackQuery      *CallbackQuery      `json:"callback_query"`
}

type BusinessConnection struct {
	ID         string `json:"id"`
	UserChatID int64  `json:"user_chat_id"`
	IsEnabled  bool   `json:"is_enabled"`
}

type Message struct {
	MessageID            int64  `json:"message_id"`
	From                 *User  `json:"from"`
	Chat                 *Chat  `json:"chat"`
	Text                 string `json:"text"`
	BusinessConnectionID string `json:"business_connection_id"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	IsPremium bool   `json:"is_premium"` // Menangkap field is_premium dari Telegram API
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID   string   `json:"id"`
	From *User    `json:"from"`
	Data string   `json:"data"`
	Msg  *Message `json:"message"`
}

func NewTelegramClient(token string) *TelegramClient {
	return &TelegramClient{
		Token:   token,
		BaseURL: "https://api.telegram.org/bot" + token,
	}
}

func (t *TelegramClient) AnswerCallback(callbackQueryID string) {
	url := t.BaseURL + "/answerCallbackQuery"
	payload := map[string]interface{}{"callback_query_id": callbackQueryID}
	jsonData, _ := json.Marshal(payload)
	http.Post(url, "application/json", bytes.NewBuffer(jsonData))
}

func (t *TelegramClient) DeleteMessage(chatID int64, messageID int64) {
	url := t.BaseURL + "/deleteMessage"
	payload := map[string]interface{}{"chat_id": chatID, "message_id": messageID}
	jsonData, _ := json.Marshal(payload)
	http.Post(url, "application/json", bytes.NewBuffer(jsonData))
}

func (t *TelegramClient) SendMessage(chatID int64, text string, businessConnID string, markup interface{}) (int64, error) {
	url := t.BaseURL + "/sendMessage"
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if businessConnID != "" {
		payload["business_connection_id"] = businessConnID
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var res struct {
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	return res.Result.MessageID, nil
}

func (t *TelegramClient) EditMessage(chatID int64, messageID int64, text string, markup interface{}) {
	url := t.BaseURL + "/editMessageText"
	payload := map[string]interface{}{
		"chat_id":      chatID,
		"message_id":   messageID,
		"text":         text,
		"parse_mode":   "HTML",
		"reply_markup": markup,
	}
	jsonData, _ := json.Marshal(payload)
	http.Post(url, "application/json", bytes.NewBuffer(jsonData))
}
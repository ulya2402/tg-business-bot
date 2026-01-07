package handlers

import (
	"fmt"
	"strings"
	"tg-business-bot/internal/api"
	"tg-business-bot/internal/database"
	"tg-business-bot/internal/encryption"
	"tg-business-bot/internal/i18n"
	"tg-business-bot/internal/models"
)

type BotHandler struct {
	DB         *database.SupabaseClient
	TG         *api.TelegramClient
	I18n       *i18n.Bundle
	EncryptKey string
}

func NewBotHandler(db *database.SupabaseClient, tg *api.TelegramClient, i18n *i18n.Bundle, encKey string) *BotHandler {
	return &BotHandler{DB: db, TG: tg, I18n: i18n, EncryptKey: encKey}
}

func (h *BotHandler) HandleUpdate(update api.Update) {
	if update.CallbackQuery != nil {
		h.handleCallbackQuery(update.CallbackQuery)
		return
	}
	if update.BusinessMessage != nil {
		h.handleBusinessMessage(update.BusinessMessage)
		return
	}
	if update.Message != nil {
		h.handlePrivateMessage(update.Message)
		return
	}
	if update.BusinessConnection != nil {
		h.handleBusinessConnection(update.BusinessConnection)
		return
	}
}

func (h *BotHandler) handlePrivateMessage(msg *api.Message) {
	user, _ := h.DB.GetUser(msg.From.ID)
	if user == nil {
		h.DB.UpsertUser(models.User{TelegramID: msg.From.ID, Language: "en", AIModel: "openai/gpt-oss-120b", SystemPrompt: "You are a professional assistant."})
		user, _ = h.DB.GetUser(msg.From.ID)
	}

	if msg.Text == "/settings" {
		if user.LastDashboardID != 0 { h.TG.DeleteMessage(msg.Chat.ID, user.LastDashboardID) }
		msgID, _ := h.TG.SendMessage(msg.Chat.ID, h.getDashboardText(user, ""), "", h.getDashboardMarkup(user))
		user.LastDashboardID = msgID
		h.DB.UpsertUser(*user)
		return
	}

	if strings.HasPrefix(user.SystemPrompt, "WAIT_FOR_PROMPT:") {
		user.SystemPrompt = msg.Text
		h.DB.UpsertUser(*user)
		h.TG.DeleteMessage(msg.Chat.ID, msg.MessageID)
		h.TG.EditMessage(msg.Chat.ID, user.LastDashboardID, h.getDashboardText(user, "✅ <b>Prompt updated successfully!</b>"), h.getDashboardMarkup(user))
		return
	}

	if strings.HasPrefix(msg.Text, "gsk_") {
		enc, _ := encryption.Encrypt(msg.Text, h.EncryptKey)
		user.EncryptedGroqKey = enc
		h.DB.UpsertUser(*user)
		h.TG.SendMessage(msg.Chat.ID, "✅ <b>API Key Saved!</b>", "", nil)
	}
}

func (h *BotHandler) handleBusinessMessage(msg *api.Message) {
	owner, _ := h.DB.GetUserByBusinessConnID(msg.BusinessConnectionID)
	if owner == nil || owner.EncryptedGroqKey == "" { return }

	realName := "Unknown"
	if msg.From != nil {
		if msg.From.FirstName != "" {
			realName = msg.From.FirstName
			if msg.From.LastName != "" { realName += " " + msg.From.LastName }
		} else if msg.From.Username != "" {
			realName = msg.From.Username
		}
	}

	h.DB.SaveMessage(owner.TelegramID, msg.Chat.ID, realName, "user", msg.Text)
	history := h.DB.GetChatHistory(owner.TelegramID, msg.Chat.ID)
	
	var final []models.ChatMessage
	final = append(final, models.ChatMessage{Role: "system", Content: owner.SystemPrompt})
	final = append(final, history...)

	key, _ := encryption.Decrypt(owner.EncryptedGroqKey, h.EncryptKey)
	groq := api.NewGroqClient(key)
	resp, _ := groq.GetChatCompletion(owner.AIModel, final)

	h.DB.SaveMessage(owner.TelegramID, msg.Chat.ID, realName, "assistant", resp)
	h.TG.SendMessage(msg.Chat.ID, resp, msg.BusinessConnectionID, nil)
}

func (h *BotHandler) handleCallbackQuery(cb *api.CallbackQuery) {
	h.TG.AnswerCallback(cb.ID)
	user, _ := h.DB.GetUser(cb.From.ID)
	if user == nil { return }

	if cb.Data == "menu_model" {
		markup := map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{{"text": "GPT-OSS 120B", "callback_data": "set_model_openai/gpt-oss-120b"}},
				{{"text": "Llama 4 Maverick", "callback_data": "set_model_meta-llama/llama-4-maverick-17b-128e-instruct"}},
				{{"text": "« Back", "callback_data": "back_main"}},
			},
		}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, "<b>Select AI Model:</b>", markup)
	} else if strings.HasPrefix(cb.Data, "set_model_") {
		user.AIModel = strings.TrimPrefix(cb.Data, "set_model_")
		h.DB.UpsertUser(*user)
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.getDashboardText(user, "✅ <b>Model updated!</b>"), h.getDashboardMarkup(user))
	} else if cb.Data == "menu_prompt" {
		oldPrompt := user.SystemPrompt
		user.SystemPrompt = "WAIT_FOR_PROMPT:" + oldPrompt
		h.DB.UpsertUser(*user)
		markup := map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{{"text": "« Cancel", "callback_data": "cancel_prompt"}},
			},
		}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, "📥 <b>Send a new System Prompt:</b>\n\n<i>Your next text message will be saved as the new AI instruction.</i>", markup)
	} else if cb.Data == "cancel_prompt" {
		if strings.HasPrefix(user.SystemPrompt, "WAIT_FOR_PROMPT:") {
			user.SystemPrompt = strings.TrimPrefix(user.SystemPrompt, "WAIT_FOR_PROMPT:")
			h.DB.UpsertUser(*user)
		}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.getDashboardText(user, ""), h.getDashboardMarkup(user))
	} else if cb.Data == "menu_clear_list" {
		customers := h.DB.GetBusinessCustomers(user.TelegramID)
		var buttons [][]map[string]interface{}
		if len(customers) == 0 {
			h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, "❌ <b>No chat history found.</b>", map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": "« Back", "callback_data": "back_main"}}}})
			return
		}
		for _, c := range customers {
			cid := int64(c["customer_id"].(float64))
			name := c["customer_name"].(string)
			buttons = append(buttons, []map[string]interface{}{{"text": "👤 " + name, "callback_data": fmt.Sprintf("confirm_clear_%d", cid)}})
		}
		buttons = append(buttons, []map[string]interface{}{{"text": "« Back", "callback_data": "back_main"}})
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, "<b>Select Customer to Clear History:</b>", map[string]interface{}{"inline_keyboard": buttons})
	} else if strings.HasPrefix(cb.Data, "confirm_clear_") {
		cid := strings.TrimPrefix(cb.Data, "confirm_clear_")
		markup := map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{{"text": "⚠️ YES, DELETE", "callback_data": "exec_clear_" + cid}},
				{{"text": "« Cancel", "callback_data": "menu_clear_list"}},
			},
		}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, "<b>⚠️ WARNING</b>\nDelete history for this customer? This cannot be undone.", markup)
	} else if strings.HasPrefix(cb.Data, "exec_clear_") {
		cid := strings.TrimPrefix(cb.Data, "exec_clear_")
		var targetID int64
		fmt.Sscanf(cid, "%d", &targetID)
		h.DB.ClearHistoryPerUser(user.TelegramID, targetID)
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, "✅ <b>History Cleared!</b>", map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": "« Back", "callback_data": "menu_clear_list"}}}})
	} else if cb.Data == "back_main" {
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.getDashboardText(user, ""), h.getDashboardMarkup(user))
	}
}

func (h *BotHandler) getDashboardText(user *models.User, status string) string {
	p := user.SystemPrompt
	if strings.HasPrefix(p, "WAIT_FOR_PROMPT:") { p = strings.TrimPrefix(p, "WAIT_FOR_PROMPT:") }
	p = strings.ReplaceAll(p, "<", "&lt;")
	p = strings.ReplaceAll(p, ">", "&gt;")
	
	header := "<b>🛠 DASHBOARD SETTINGS</b>"
	if status != "" { header = status + "\n\n" + header }
	
	return fmt.Sprintf("%s\n\n<b>🤖 Model:</b>\n<code>%s</code>\n\n<b>📝 Prompt:</b>\n<code>%s</code>", header, user.AIModel, p)
}

func (h *BotHandler) getDashboardMarkup(user *models.User) map[string]interface{} {
	return map[string]interface{}{
		"inline_keyboard": [][]map[string]interface{}{
			{{"text": "🤖 AI Model", "callback_data": "menu_model"}, {"text": "📝 Edit Prompt", "callback_data": "menu_prompt"}},
			{{"text": "🧹 Clear Chat History", "callback_data": "menu_clear_list"}},
		},
	}
}

func (h *BotHandler) handleBusinessConnection(conn *api.BusinessConnection) {
	user, _ := h.DB.GetUser(conn.UserChatID)
	if user != nil {
		user.BusinessConnID = conn.ID
		h.DB.UpsertUser(*user)
	}
}
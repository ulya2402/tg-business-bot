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

const MasterHTMLPrompt = `You are a professional assistant for a Telegram Business account. 
CRITICAL RULE: You MUST use Telegram-compatible HTML for formatting.
Supported tags: <b>bold</b>, <i>italic</i>, <u>underline</u>, <s>strikethrough</s>, <code>code</code>, <pre>preformatted</pre>, <a href="URL">link</a>.
Do NOT use Markdown. Ensure all tags are properly closed. For new lines, just use a normal line break.`

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
}

// --- FUNGSI BARU: refreshDashboard ---
// Fungsi ini memastikan dashboard selalu diperbarui pada pesan yang tepat.
func (h *BotHandler) refreshDashboard(chatID int64, user *models.User, status string) {
	text := h.getDashboardText(user, status)
	markup := h.getDashboardMarkup(user)

	if user.LastDashboardID != 0 {
		h.TG.EditMessage(chatID, user.LastDashboardID, text, markup)
	} else {
		// Fallback jika ID hilang, kirim pesan baru
		msgID, _ := h.TG.SendMessage(chatID, text, "", markup)
		user.LastDashboardID = msgID
		h.DB.UpsertUser(*user)
	}
}

func (h *BotHandler) handlePrivateMessage(msg *api.Message) {
	user, _ := h.DB.GetUser(msg.From.ID)
	if user == nil {
		h.DB.UpsertUser(models.User{TelegramID: msg.From.ID, Language: "en", AIModel: "openai/gpt-oss-120b", SystemPrompt: "You are a professional assistant.", IsPremium: msg.From.IsPremium})
		user, _ = h.DB.GetUser(msg.From.ID)
	}

	if !msg.From.IsPremium {
		h.TG.SendMessage(msg.Chat.ID, h.I18n.Get("en", "access_denied"), "", nil)
		return
	}

	lang := user.Language

	if msg.Text == "/start" {
		markup := map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{{"text": h.I18n.Get(lang, "btn_set_key"), "callback_data": "menu_key"}},
				{{"text": h.I18n.Get(lang, "btn_dashboard"), "callback_data": "back_main"}},
			},
		}
		h.TG.SendMessage(msg.Chat.ID, h.I18n.Get(lang, "welcome"), "", markup)
		return
	}

	if msg.Text == "/settings" {
		if user.LastDashboardID != 0 {
			h.TG.DeleteMessage(msg.Chat.ID, user.LastDashboardID)
		}
		msgID, _ := h.TG.SendMessage(msg.Chat.ID, h.getDashboardText(user, ""), "", h.getDashboardMarkup(user))
		user.LastDashboardID = msgID
		h.DB.UpsertUser(*user)
		return
	}

	// Logic input System Prompt
	if strings.HasPrefix(user.SystemPrompt, "WAIT_FOR_PROMPT:") {
		user.SystemPrompt = msg.Text
		h.DB.UpsertUser(*user)
		h.TG.DeleteMessage(msg.Chat.ID, msg.MessageID)
		h.refreshDashboard(msg.Chat.ID, user, "✅ <b>Prompt Updated!</b>")
		return
	}

	// Logic input Groq Key
	if strings.HasPrefix(user.EncryptedGroqKey, "WAIT_FOR_KEY:") {
		h.TG.DeleteMessage(msg.Chat.ID, msg.MessageID)
		
		if !strings.HasPrefix(msg.Text, "gsk_") {
			user.EncryptedGroqKey = "" // Reset state
			h.DB.UpsertUser(*user)
			h.refreshDashboard(msg.Chat.ID, user, h.I18n.Get(lang, "key_invalid"))
			return
		}

		enc, _ := encryption.Encrypt(msg.Text, h.EncryptKey)
		user.EncryptedGroqKey = enc
		h.DB.UpsertUser(*user)
		h.refreshDashboard(msg.Chat.ID, user, h.I18n.Get(lang, "key_success"))
		return
	}
}

func (h *BotHandler) handleBusinessMessage(msg *api.Message) {
	owner, _ := h.DB.GetUserByBusinessConnID(msg.BusinessConnectionID)
	if owner == nil || owner.EncryptedGroqKey == "" || strings.HasPrefix(owner.EncryptedGroqKey, "WAIT_") {
		return
	}

	displayName := "Customer"
	if msg.From != nil {
		if msg.From.Username != "" {
			displayName = "@" + msg.From.Username
		} else if msg.From.FirstName != "" {
			displayName = msg.From.FirstName
			if msg.From.LastName != "" {
				displayName += " " + msg.From.LastName
			}
		} else {
			displayName = fmt.Sprintf("User %d", msg.From.ID)
		}
	}

	h.DB.SaveMessage(owner.TelegramID, msg.Chat.ID, displayName, "user", msg.Text)
	history := h.DB.GetChatHistory(owner.TelegramID, msg.Chat.ID)
	
	var final []models.ChatMessage
	combinedPrompt := MasterHTMLPrompt + "\n\nBusiness Context: " + owner.SystemPrompt
	final = append(final, models.ChatMessage{Role: "system", Content: combinedPrompt})
	final = append(final, history...)

	key, _ := encryption.Decrypt(owner.EncryptedGroqKey, h.EncryptKey)
	groq := api.NewGroqClient(key)
	resp, _ := groq.GetChatCompletion(owner.AIModel, final)

	h.DB.SaveMessage(owner.TelegramID, msg.Chat.ID, displayName, "assistant", resp)
	h.TG.SendMessage(msg.Chat.ID, resp, msg.BusinessConnectionID, nil)
}

func (h *BotHandler) handleCallbackQuery(cb *api.CallbackQuery) {
	h.TG.AnswerCallback(cb.ID)
	user, _ := h.DB.GetUser(cb.From.ID)
	if user == nil { return }

	// --- PERBAIKAN LOGIKA: Selalu gunakan ID pesan saat ini sebagai Dashboard ---
	user.LastDashboardID = cb.Msg.MessageID
	h.DB.UpsertUser(*user)

	lang := user.Language
	
	if cb.Data == "menu_model" {
		markup := map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": "GPT-OSS 120B", "callback_data": "set_model_openai/gpt-oss-120b"}}, {{"text": "Llama 4 Maverick", "callback_data": "set_model_meta-llama/llama-4-maverick-17b-128e-instruct"}}, {{"text": h.I18n.Get(lang, "btn_back"), "callback_data": "back_main"}}}}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.I18n.Get(lang, "select_model"), markup)
	} else if strings.HasPrefix(cb.Data, "set_model_") {
		user.AIModel = strings.TrimPrefix(cb.Data, "set_model_")
		h.DB.UpsertUser(*user)
		h.refreshDashboard(cb.From.ID, user, "")
	} else if cb.Data == "menu_prompt" {
		user.SystemPrompt = "WAIT_FOR_PROMPT:"
		h.DB.UpsertUser(*user)
		markup := map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": h.I18n.Get(lang, "btn_cancel"), "callback_data": "back_main"}}}}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.I18n.Get(lang, "prompt_input"), markup)
	} else if cb.Data == "menu_key" {
		user.EncryptedGroqKey = "WAIT_FOR_KEY:"
		h.DB.UpsertUser(*user)
		markup := map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": h.I18n.Get(lang, "btn_cancel"), "callback_data": "back_main"}}}}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.I18n.Get(lang, "key_input"), markup)
	} else if cb.Data == "menu_clear_list" {
		customers := h.DB.GetBusinessCustomers(user.TelegramID)
		var buttons [][]map[string]interface{}
		if len(customers) == 0 {
			h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.I18n.Get(lang, "no_history"), map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": h.I18n.Get(lang, "btn_back"), "callback_data": "back_main"}}}})
			return
		}
		for _, c := range customers {
			var cid int64
			if v, ok := c["customer_id"].(int64); ok { cid = v } else if v, ok := c["customer_id"].(float64); ok { cid = int64(v) }
			name := c["customer_name"].(string)
			buttons = append(buttons, []map[string]interface{}{{"text": "👤 " + name, "callback_data": fmt.Sprintf("confirm_clear_%d", cid)}})
		}
		buttons = append(buttons, []map[string]interface{}{{"text": h.I18n.Get(lang, "btn_back"), "callback_data": "back_main"}})
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.I18n.Get(lang, "clear_list"), map[string]interface{}{"inline_keyboard": buttons})
	} else if strings.HasPrefix(cb.Data, "confirm_clear_") {
		cid := strings.TrimPrefix(cb.Data, "confirm_clear_")
		markup := map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": h.I18n.Get(lang, "btn_delete_confirm"), "callback_data": "exec_clear_" + cid}}, {{"text": h.I18n.Get(lang, "btn_cancel"), "callback_data": "menu_clear_list"}}}}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.I18n.Get(lang, "clear_warn"), markup)
	} else if strings.HasPrefix(cb.Data, "exec_clear_") {
		cid := strings.TrimPrefix(cb.Data, "exec_clear_")
		var targetID int64
		fmt.Sscanf(cid, "%d", &targetID)
		h.DB.ClearHistoryPerUser(user.TelegramID, targetID)
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, h.I18n.Get(lang, "history_cleared"), map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": h.I18n.Get(lang, "btn_back"), "callback_data": "menu_clear_list"}}}})
	} else if cb.Data == "back_main" {
		// Reset state jika membatalkan input
		if strings.HasPrefix(user.SystemPrompt, "WAIT_") { user.SystemPrompt = "Assistant." }
		if strings.HasPrefix(user.EncryptedGroqKey, "WAIT_") { user.EncryptedGroqKey = "" }
		h.DB.UpsertUser(*user)
		h.refreshDashboard(cb.From.ID, user, "")
	} else if cb.Data == "menu_lang" {
		markup := map[string]interface{}{"inline_keyboard": [][]map[string]interface{}{{{"text": "English 🇺🇸", "callback_data": "set_lang_en"}, {"text": "Indonesia 🇮🇩", "callback_data": "set_lang_id"}}, {{"text": "Russian 🇷🇺", "callback_data": "set_lang_ru"}}, {{"text": h.I18n.Get(lang, "btn_back"), "callback_data": "back_main"}}}}
		h.TG.EditMessage(cb.From.ID, cb.Msg.MessageID, "<b>Select Language</b>", markup)
	} else if strings.HasPrefix(cb.Data, "set_lang_") {
		user.Language = strings.TrimPrefix(cb.Data, "set_lang_")
		h.DB.UpsertUser(*user)
		h.refreshDashboard(cb.From.ID, user, "")
	}
}

func (h *BotHandler) getDashboardText(user *models.User, status string) string {
	lang := user.Language
	p := user.SystemPrompt
	if strings.HasPrefix(p, "WAIT_") { p = h.I18n.Get(lang, "wait_input") }
	p = strings.ReplaceAll(strings.ReplaceAll(p, "<", "&lt;"), ">", "&gt;")
	
	keyStatus := h.I18n.Get(lang, "key_not_set")
	if user.EncryptedGroqKey != "" && !strings.HasPrefix(user.EncryptedGroqKey, "WAIT_") { keyStatus = h.I18n.Get(lang, "key_set") }
	
	header := h.I18n.Get(lang, "dash_title")
	if status != "" { 
		header = status + "\n\n" + header 
	}

	return fmt.Sprintf("%s\n\n%s <code>%s</code>\n%s <code>%s</code>\n%s <code>%s</code>", 
		header, 
		h.I18n.Get(lang, "dash_model"), user.AIModel, 
		h.I18n.Get(lang, "dash_key"), keyStatus, 
		h.I18n.Get(lang, "dash_prompt"), p)
}

func (h *BotHandler) getDashboardMarkup(user *models.User) map[string]interface{} {
	lang := user.Language
	return map[string]interface{}{
		"inline_keyboard": [][]map[string]interface{}{
			{{"text": h.I18n.Get(lang, "btn_model"), "callback_data": "menu_model"}, {"text": h.I18n.Get(lang, "btn_prompt"), "callback_data": "menu_prompt"}},
			{{"text": h.I18n.Get(lang, "btn_update_key"), "callback_data": "menu_key"}},
			{{"text": h.I18n.Get(lang, "btn_clear_history"), "callback_data": "menu_clear_list"}},
			{{"text": "🌐 Language", "callback_data": "menu_lang"}},
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
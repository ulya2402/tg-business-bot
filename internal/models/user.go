package models

type User struct {
	TelegramID      int64  `json:"telegram_id"`
	Username        string `json:"username"`
	Language        string `json:"language"`
	EncryptedGroqKey string `json:"encrypted_groq_key"`
	IsPremium       bool   `json:"is_premium"`
	BusinessConnID  string `json:"business_connection_id"`
	SystemPrompt    string `json:"system_prompt"`
	AIModel         string `json:"ai_model"`
	LastDashboardID int64  `json:"last_dashboard_id"`
	CreatedAt       string `json:"created_at,omitempty"`
	BusinessLocation string `json:"business_location"`
    Latitude         float64 `json:"latitude"`
    Longitude        float64 `json:"longitude"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
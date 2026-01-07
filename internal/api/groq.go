package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"tg-business-bot/internal/models"
)

type GroqClient struct {
	APIKey string
}

func NewGroqClient(apiKey string) *GroqClient {
	return &GroqClient{APIKey: apiKey}
}

func (g *GroqClient) GetChatCompletion(model string, history []models.ChatMessage) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"
	
	payload := map[string]interface{}{
		"model":    model,
		"messages": history,
	}
	
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+g.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Groq Error: %s", string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	json.Unmarshal(body, &result)
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("empty response")
}
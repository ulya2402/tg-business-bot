package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"tg-business-bot/internal/models"
)

type SupabaseClient struct {
	URL string
	Key string
}

func NewSupabaseClient(url, key string) *SupabaseClient {
	return &SupabaseClient{URL: url, Key: key}
}

func (s *SupabaseClient) GetUser(telegramID int64) (*models.User, error) {
	url := fmt.Sprintf("%s/rest/v1/users?telegram_id=eq.%d&select=*", s.URL, telegramID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", s.Key)
	req.Header.Set("Authorization", "Bearer "+s.Key)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var users []models.User
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &users)
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func (s *SupabaseClient) GetUserByBusinessConnID(connID string) (*models.User, error) {
	url := fmt.Sprintf("%s/rest/v1/users?business_connection_id=eq.%s&select=*", s.URL, connID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", s.Key)
	req.Header.Set("Authorization", "Bearer "+s.Key)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var users []models.User
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &users)
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

func (s *SupabaseClient) UpsertUser(user models.User) error {
	url := fmt.Sprintf("%s/rest/v1/users", s.URL)
	jsonData, _ := json.Marshal(user)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", s.Key)
	req.Header.Set("Authorization", "Bearer "+s.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates,return=minimal")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (s *SupabaseClient) SaveMessage(ownerID, customerID int64, customerName, role, content string) {
	url := fmt.Sprintf("%s/rest/v1/messages", s.URL)
	payload := map[string]interface{}{"owner_id": ownerID, "customer_id": customerID, "customer_name": customerName, "role": role, "content": content}
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", s.Key)
	req.Header.Set("Authorization", "Bearer "+s.Key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("DB Error: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (s *SupabaseClient) GetChatHistory(ownerID, customerID int64) []models.ChatMessage {
	url := fmt.Sprintf("%s/rest/v1/messages?owner_id=eq.%d&customer_id=eq.%d&order=created_at.desc&limit=10", s.URL, ownerID, customerID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", s.Key)
	req.Header.Set("Authorization", "Bearer "+s.Key)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var rawMsgs []models.ChatMessage
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &rawMsgs)
	var history []models.ChatMessage
	for i := len(rawMsgs) - 1; i >= 0; i-- {
		history = append(history, rawMsgs[i])
	}
	return history
}

func (s *SupabaseClient) GetBusinessCustomers(ownerID int64) []map[string]interface{} {
	url := fmt.Sprintf("%s/rest/v1/messages?owner_id=eq.%d&select=customer_id,customer_name", s.URL, ownerID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", s.Key)
	req.Header.Set("Authorization", "Bearer "+s.Key)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var results []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &results)
	keys := make(map[int64]bool)
	var list []map[string]interface{}
	for _, entry := range results {
		cid := int64(entry["customer_id"].(float64))
		if _, value := keys[cid]; !value {
			keys[cid] = true
			list = append(list, entry)
		}
	}
	return list
}

func (s *SupabaseClient) ClearHistoryPerUser(ownerID, customerID int64) {
	url := fmt.Sprintf("%s/rest/v1/messages?owner_id=eq.%d&customer_id=eq.%d", s.URL, ownerID, customerID)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("apikey", s.Key)
	req.Header.Set("Authorization", "Bearer "+s.Key)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("DB Error: %v", err)
		return
	}
	defer resp.Body.Close()
}
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"tg-business-bot/internal/api"
	"tg-business-bot/internal/database"
	"tg-business-bot/internal/handlers"
	"tg-business-bot/internal/i18n"
	"time"
)

func main() {
	cfg := LoadConfig()

	db := database.NewSupabaseClient(cfg.SupabaseURL, cfg.SupabaseServiceKey)
	tg := api.NewTelegramClient(cfg.BotToken)
	
	bundle := i18n.NewBundle()
	if err := bundle.LoadLocale("en", "locales/en.json"); err != nil {
		log.Fatalf("Failed to load English locale: %v", err)
	}
	if err := bundle.LoadLocale("id", "locales/id.json"); err != nil {
		log.Fatalf("Failed to load Indonesian locale: %v", err)
	}

	handler := handlers.NewBotHandler(db, tg, bundle, cfg.EncryptionKey)

	log.Println("Bot Engine Started: Polling for updates...")

	offset := int64(0)
	for {
		updates, err := getUpdates(cfg.BotToken, offset)
		if err != nil {
			log.Printf("Polling Error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			handler.HandleUpdate(update)
			offset = update.UpdateID + 1
		}
	}
}

func getUpdates(token string, offset int64) ([]api.Update, error) {
	apiUrl := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", token, offset)
	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool         `json:"ok"`
		Result []api.Update `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Result, nil
}
package main

import (
	"bufio"
	"log"
	"os"
	"strings"
)

type Config struct {
	BotToken           string
	SupabaseURL        string
	SupabaseServiceKey string
	GroqMasterKey      string
	EncryptionKey      string
	Port               string
	Debug              bool
}

func loadEnvFile() {
	file, err := os.Open(".env")
	if err != nil {
		log.Printf("System: No .env file found, using system environment variables")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		os.Setenv(key, val)
	}
}

func LoadConfig() *Config {
	loadEnvFile()

	conf := &Config{
		BotToken:           os.Getenv("BOT_TOKEN"),
		SupabaseURL:        os.Getenv("SUPABASE_URL"),
		SupabaseServiceKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		GroqMasterKey:      os.Getenv("GROQ_API_KEY"),
		EncryptionKey:      os.Getenv("ENCRYPTION_KEY"),
		Port:               os.Getenv("PORT"),
		Debug:              os.Getenv("DEBUG") == "true",
	}

	if conf.BotToken == "" || conf.EncryptionKey == "" || conf.SupabaseURL == "" {
		log.Fatalf("Critical Error: Missing required variables in .env. Check BOT_TOKEN, ENCRYPTION_KEY, and SUPABASE_URL")
	}

	if len(conf.EncryptionKey) != 32 {
		log.Fatalf("Critical Error: ENCRYPTION_KEY must be exactly 32 characters. Current length: %d", len(conf.EncryptionKey))
	}

	log.Printf("System: Configuration loaded successfully. Debug: %v", conf.Debug)
	return conf
}
package i18n

import (
	"encoding/json"
	"os"
)

type Bundle struct {
	locales map[string]map[string]string
}

func NewBundle() *Bundle {
	return &Bundle{
		locales: make(map[string]map[string]string),
	}
}

func (b *Bundle) LoadLocale(langCode string, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return err
	}

	b.locales[langCode] = messages
	return nil
}

func (b *Bundle) Get(lang string, key string) string {
	if messages, ok := b.locales[lang]; ok {
		if val, ok := messages[key]; ok {
			return val
		}
	}
	
	if messages, ok := b.locales["en"]; ok {
		if val, ok := messages[key]; ok {
			return val
		}
	}
	
	return key
}
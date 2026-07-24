package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken        string
	ChannelID       int64
	ChannelUsername string
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()

	chID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHANNEL_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("TELEGRAM_CHANNEL_ID 无效: %w", err)
	}

	cfg := &Config{
		BotToken:        os.Getenv("TELEGRAM_BOT_TOKEN"),
		ChannelID:       chID,
		ChannelUsername: strings.TrimPrefix(os.Getenv("TELEGRAM_CHANNEL_USERNAME"), "@"),
	}
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("缺少 TELEGRAM_BOT_TOKEN")
	}
	return cfg, nil
}

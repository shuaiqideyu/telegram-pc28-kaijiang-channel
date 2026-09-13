package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken  string
	ChannelID int64
	Yu28Base  string
	Yu28Key   string
	DebugDM   bool
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()

	chID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHANNEL_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("TELEGRAM_CHANNEL_ID 无效: %w", err)
	}

	cfg := &Config{
		BotToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		ChannelID: chID,
		Yu28Base:  strings.TrimRight(strings.TrimSpace(os.Getenv("YU28_BASE")), "/"),
		Yu28Key:   strings.TrimSpace(os.Getenv("YU28_API_KEY")),
		DebugDM:   parseBoolEnv(os.Getenv("DEBUG_DM")),
	}
	if cfg.Yu28Base == "" {
		cfg.Yu28Base = "https://yu28.top"
	}
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("缺少 TELEGRAM_BOT_TOKEN")
	}
	if cfg.Yu28Key == "" {
		return nil, fmt.Errorf("缺少 YU28_API_KEY")
	}
	return cfg, nil
}

func parseBoolEnv(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

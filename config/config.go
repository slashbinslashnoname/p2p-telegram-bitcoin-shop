package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration
type Config struct {
	TelegramToken string
	BTCPayURL     string
	BTCPayAPIKey  string
	BTCPayStoreID string
	DBPath        string
}

// NewConfig creates a new configuration from environment variables
func NewConfig() *Config {
	// Load .env file if it exists
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using default values")
	}

	return &Config{
		TelegramToken: getEnv("TELEGRAM_BOT_TOKEN", "YOUR_TELEGRAM_BOT_TOKEN"),
		BTCPayURL:     getEnv("BTCPAY_URL", "https://your.btcpayserver.com"),
		BTCPayAPIKey:  getEnv("BTCPAY_API_KEY", "YOUR_BTCPAY_API_KEY"),
		BTCPayStoreID: getEnv("BTCPAY_STORE_ID", "YOUR_BTCPAY_STORE_ID"),
		DBPath:        getEnv("DB_PATH", "./btc_trades.db"),
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
} 
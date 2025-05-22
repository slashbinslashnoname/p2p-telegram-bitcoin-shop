package main

import (
	"log"

	_ "github.com/mattn/go-sqlite3"
	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/bot"
	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/config"
)

// P2P Telegram Bitcoin Shop is an interactive Telegram bot that allows users to sell Bitcoin
// via Lightning Network using BTCPay Server. It provides a user-friendly interface with
// buttons for easier navigation and formatted messages for better readability.
func main() {
	// Load configuration
	cfg := config.NewConfig()

	// Initialize and start the bot
	telegramBot, err := bot.NewBot(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	log.Println("Bot started...")
	telegramBot.Start()
}
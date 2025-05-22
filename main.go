package main

import (
	"log"

	_ "github.com/mattn/go-sqlite3"
	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/bot"
	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/config"
)

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
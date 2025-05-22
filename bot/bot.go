package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/btcpay"
	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/config"
	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/db"
	"gopkg.in/tucnak/telebot.v2"
)

// Bot represents the Telegram bot with its dependencies
type Bot struct {
	teleBot   *telebot.Bot
	database  *db.Database
	btcpay    *btcpay.Client
	config    *config.Config
}

// NewBot creates a new Bot instance
func NewBot(cfg *config.Config) (*Bot, error) {
	database, err := db.NewDatabase(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	bot, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.TelegramToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %v", err)
	}

	btcpayClient := btcpay.NewClient(cfg.BTCPayURL, cfg.BTCPayAPIKey, cfg.BTCPayStoreID)

	return &Bot{
		teleBot:  bot,
		database: database,
		btcpay:   btcpayClient,
		config:   cfg,
	}, nil
}

// registerUser registers a new user in the database
func (b *Bot) registerUser(m *telebot.Message) error {
	if err := b.database.RegisterUser(m.Sender.ID, m.Sender.Username); err != nil {
		return err
	}
	b.teleBot.Send(m.Sender, "Successfully registered!")
	return nil
}

// createOffer creates a new Bitcoin selling offer
func (b *Bot) createOffer(m *telebot.Message, amountBTC, priceUSD float64) error {
	// Verify user exists
	exists, err := b.database.UserExists(m.Sender.ID)
	if err != nil || !exists {
		b.teleBot.Send(m.Sender, "Please register first with /start")
		return nil
	}

	// Calculate amount in satoshis (1 BTC = 100,000,000 sats)
	amountSats := int64(amountBTC * 100_000_000)

	// Create BTCPay Server invoice
	invoiceID, invoiceLink, err := b.btcpay.CreateInvoice(amountSats, fmt.Sprintf("BTC sell offer by %d", m.Sender.ID))
	if err != nil {
		b.teleBot.Send(m.Sender, "Failed to create Lightning invoice")
		return fmt.Errorf("failed to create invoice: %v", err)
	}

	// Store offer
	if err := b.database.CreateOffer(m.Sender.ID, amountBTC, priceUSD, invoiceID, invoiceLink); err != nil {
		b.teleBot.Send(m.Sender, "Failed to create offer")
		return fmt.Errorf("failed to create offer: %v", err)
	}

	b.teleBot.Send(m.Sender, fmt.Sprintf("Offer created!\nAmount: %f BTC\nPrice: $%f\nLightning Invoice: %s", amountBTC, priceUSD, invoiceLink))
	return nil
}

// listOffers lists all offers for a user
func (b *Bot) listOffers(m *telebot.Message) error {
	offers, err := b.database.GetUserOffers(m.Sender.ID)
	if err != nil {
		b.teleBot.Send(m.Sender, "Failed to fetch offers")
		return fmt.Errorf("failed to fetch offers: %v", err)
	}

	if len(offers) == 0 {
		b.teleBot.Send(m.Sender, "No offers found")
		return nil
	}

	var response strings.Builder
	response.WriteString("Your offers:\n")
	for _, o := range offers {
		// Check payment status
		paid, err := b.btcpay.CheckInvoiceStatus(o.InvoiceID)
		if err != nil {
			log.Printf("Failed to check invoice status for offer %d: %v", o.ID, err)
		}
		status := "Pending"
		if paid {
			status = "Paid"
		}
		response.WriteString(fmt.Sprintf("ID: %d | %f BTC | $%f | %s | %s\n", o.ID, o.AmountBTC, o.PriceUSD, o.CreatedAt.Format(time.RFC822), status))
	}
	b.teleBot.Send(m.Sender, response.String())
	return nil
}

// Start starts the bot and registers command handlers
func (b *Bot) Start() {
	b.teleBot.Handle("/start", func(m *telebot.Message) {
		if err := b.registerUser(m); err != nil {
			log.Printf("Error registering user: %v", err)
		}
	})

	b.teleBot.Handle("/sell", func(m *telebot.Message) {
		args := strings.Fields(m.Text)
		if len(args) != 3 {
			b.teleBot.Send(m.Sender, "Usage: /sell <amount_btc> <price_usd>")
			return
		}

		amountBTC, err := strconv.ParseFloat(args[1], 64)
		if err != nil || amountBTC <= 0 {
			b.teleBot.Send(m.Sender, "Invalid BTC amount")
			return
		}

		priceUSD, err := strconv.ParseFloat(args[2], 64)
		if err != nil || priceUSD <= 0 {
			b.teleBot.Send(m.Sender, "Invalid USD price")
			return
		}

		if err := b.createOffer(m, amountBTC, priceUSD); err != nil {
			log.Printf("Error creating offer: %v", err)
		}
	})

	b.teleBot.Handle("/list", func(m *telebot.Message) {
		if err := b.listOffers(m); err != nil {
			log.Printf("Error listing offers: %v", err)
		}
	})

	b.teleBot.Start()
} 
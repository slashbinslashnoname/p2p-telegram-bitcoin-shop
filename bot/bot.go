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
	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/models"
	"gopkg.in/tucnak/telebot.v2"
)

// Button identifiers
const (
	btnCreateOffer = "create_offer"
	btnListOffers  = "list_offers"
	btnMarketplace = "marketplace"
	btnHelp        = "help"
)

// Bot represents the Telegram bot with its dependencies
type Bot struct {
	teleBot   *telebot.Bot
	database  *db.Database
	btcpay    *btcpay.Client
	config    *config.Config
	// Button instances
	btnCreate     *telebot.InlineButton
	btnList       *telebot.InlineButton
	btnMarketplace *telebot.InlineButton
	btnHelp       *telebot.InlineButton
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

	// Create button instances
	btnCreate := telebot.InlineButton{
		Unique: btnCreateOffer,
		Text:   "🔄 Create Offer",
	}

	btnList := telebot.InlineButton{
		Unique: btnListOffers,
		Text:   "📋 My Offers",
	}
	
	btnMarketplace := telebot.InlineButton{
		Unique: btnMarketplace,
		Text:   "🛒 Marketplace",
	}

	btnHelp := telebot.InlineButton{
		Unique: btnHelp,
		Text:   "❓ Help",
	}

	return &Bot{
		teleBot:       bot,
		database:      database,
		btcpay:        btcpayClient,
		config:        cfg,
		btnCreate:     &btnCreate,
		btnList:       &btnList,
		btnMarketplace: &btnMarketplace,
		btnHelp:       &btnHelp,
	}, nil
}

// sendMainMenu sends the main menu with buttons to the user
func (b *Bot) sendMainMenu(m *telebot.Message) {
	menu := &telebot.ReplyMarkup{}
	
	// Create rows with buttons
	menu.InlineKeyboard = [][]telebot.InlineButton{
		{*b.btnCreate, *b.btnList},
		{*b.btnMarketplace},
		{*b.btnHelp},
	}

	b.teleBot.Send(m.Sender, "Welcome to P2P Bitcoin Shop! Choose an option:", menu)
}

// registerUser registers a new user in the database
func (b *Bot) registerUser(m *telebot.Message) error {
	if err := b.database.RegisterUser(m.Sender.ID, m.Sender.Username); err != nil {
		return err
	}
	
	// Send welcome message with buttons
	b.teleBot.Send(m.Sender, "Successfully registered!")
	b.sendMainMenu(m)
	
	return nil
}

// showCreateOfferForm displays the form to create a new offer
func (b *Bot) showCreateOfferForm(m *telebot.Message) {
	instructions := `To create a new offer, send a message in this format:
	
/sell <amount_btc> <price_usd>

Example: /sell 0.01 500

This will create an offer to sell 0.01 BTC for $500.`

	b.teleBot.Send(m.Sender, instructions)
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

	// Create a button to view the invoice
	menu := &telebot.ReplyMarkup{}
	btnViewInvoice := &telebot.InlineButton{
		Text: "View Invoice",
		URL:  invoiceLink,
	}
	menu.InlineKeyboard = [][]telebot.InlineButton{{*btnViewInvoice}}

	offerMsg := fmt.Sprintf("✅ Offer created!\n\n🔹 Amount: %f BTC\n🔹 Price: $%f\n\nClick the button below to view the Lightning invoice:", amountBTC, priceUSD)
	b.teleBot.Send(m.Sender, offerMsg, menu)
	
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
		b.teleBot.Send(m.Sender, "No offers found. Use the 'Create Offer' button to create your first offer.")
		return nil
	}

	var response strings.Builder
	response.WriteString("📋 *Your offers:*\n\n")
	
	// Create a menu for each offer
	for i, o := range offers {
		// Check payment status
		paid, err := b.btcpay.CheckInvoiceStatus(o.InvoiceID)
		if err != nil {
			log.Printf("Failed to check invoice status for offer %d: %v", o.ID, err)
		}
		
		status := "⏳ Pending"
		if paid {
			status = "✅ Paid"
		}
		
		// Format the offer details
		offerDetails := fmt.Sprintf(
			"*Offer #%d*\n"+
			"🔹 Amount: %f BTC\n"+
			"🔹 Price: $%f\n"+
			"🔹 Date: %s\n"+
			"🔹 Status: %s\n\n",
			o.ID, o.AmountBTC, o.PriceUSD, o.CreatedAt.Format(time.RFC822), status)
		
		response.WriteString(offerDetails)
		
		// Create a button to view the invoice for each offer
		if i < 5 { // Limit to 5 offers with buttons to avoid Telegram API limits
			menu := &telebot.ReplyMarkup{}
			btnViewInvoice := &telebot.InlineButton{
				Text: fmt.Sprintf("View Invoice #%d", o.ID),
				URL:  o.InvoiceLink,
			}
			menu.InlineKeyboard = [][]telebot.InlineButton{{*btnViewInvoice}}
			
			// Send each offer as a separate message with its own button
			b.teleBot.Send(m.Sender, offerDetails, menu, telebot.ModeMarkdown)
		}
	}
	
	// If there are more than 5 offers, send a summary message
	if len(offers) > 5 {
		b.teleBot.Send(m.Sender, fmt.Sprintf("Showing buttons for the first 5 offers. You have a total of %d offers.", len(offers)))
	}
	
	return nil
}

// showMarketplace displays all available offers from all users
func (b *Bot) showMarketplace(m *telebot.Message) error {
	// Get all offers, limit to 20 most recent
	offers, err := b.database.GetAllOffers(20)
	if err != nil {
		b.teleBot.Send(m.Sender, "Failed to fetch marketplace offers")
		return fmt.Errorf("failed to fetch marketplace offers: %v", err)
	}

	if len(offers) == 0 {
		b.teleBot.Send(m.Sender, "No offers available in the marketplace yet.")
		return nil
	}

	// Send marketplace header
	b.teleBot.Send(m.Sender, "🛒 *Bitcoin Marketplace*\n\nHere are the latest offers from all users:", telebot.ModeMarkdown)
	
	// Group offers by seller to avoid spam
	sellerOffers := make(map[int64][]models.Offer)
	for _, o := range offers {
		sellerOffers[o.UserID] = append(sellerOffers[o.UserID], o)
	}
	
	// Send offers grouped by seller
	for userID, userOffers := range sellerOffers {
		// Get the first offer to extract username
		seller := userOffers[0].Username
		if seller == "" {
			seller = fmt.Sprintf("User #%d", userID)
		}
		
		// Create a message for this seller's offers
		var sellerMsg strings.Builder
		sellerMsg.WriteString(fmt.Sprintf("👤 *Seller: @%s*\n\n", seller))
		
		// Add each offer from this seller
		for _, o := range userOffers {
			// Check payment status
			paid, err := b.btcpay.CheckInvoiceStatus(o.InvoiceID)
			if err != nil {
				log.Printf("Failed to check invoice status for offer %d: %v", o.ID, err)
			}
			
			// Skip paid offers in the marketplace
			if paid {
				continue
			}
			
			// Format the offer details
			sellerMsg.WriteString(fmt.Sprintf(
				"*Offer #%d*\n"+
				"🔹 Amount: %f BTC\n"+
				"🔹 Price: $%f\n"+
				"🔹 Date: %s\n\n",
				o.ID, o.AmountBTC, o.PriceUSD, o.CreatedAt.Format(time.RFC822)))
		}
		
		// Create contact seller button
		menu := &telebot.ReplyMarkup{}
		contactButton := &telebot.InlineButton{
			Text: fmt.Sprintf("Contact @%s", seller),
			URL:  fmt.Sprintf("https://t.me/%s", seller),
		}
		menu.InlineKeyboard = [][]telebot.InlineButton{{*contactButton}}
		
		// Send the message with the contact button
		b.teleBot.Send(m.Sender, sellerMsg.String(), menu, telebot.ModeMarkdown)
	}
	
	return nil
}

// showHelp displays help information
func (b *Bot) showHelp(m *telebot.Message) {
	helpText := `*P2P Bitcoin Shop Help*

*Available Commands:*
/start - Register as a user and show main menu
/sell <amount_btc> <price_usd> - Create a sell offer
/list - List your offers
/marketplace - Browse all available offers
/help - Show this help message

*How to use:*
1. Register with /start
2. Create an offer with /sell or use the button
3. View your offers with /list or use the button
4. Browse available offers in the marketplace

*Need more help?*
Contact support at @YourSupportUsername`

	b.teleBot.Send(m.Sender, helpText, telebot.ModeMarkdown)
}

// Start starts the bot and registers command handlers
func (b *Bot) Start() {
	// Register button handlers
	b.teleBot.Handle(&telebot.InlineButton{Unique: btnCreateOffer}, func(c *telebot.Callback) {
		b.teleBot.Respond(c, &telebot.CallbackResponse{})
		b.showCreateOfferForm(&telebot.Message{Sender: c.Sender})
	})

	b.teleBot.Handle(&telebot.InlineButton{Unique: btnListOffers}, func(c *telebot.Callback) {
		b.teleBot.Respond(c, &telebot.CallbackResponse{})
		if err := b.listOffers(&telebot.Message{Sender: c.Sender}); err != nil {
			log.Printf("Error listing offers: %v", err)
		}
	})
	
	b.teleBot.Handle(&telebot.InlineButton{Unique: btnMarketplace}, func(c *telebot.Callback) {
		b.teleBot.Respond(c, &telebot.CallbackResponse{})
		if err := b.showMarketplace(&telebot.Message{Sender: c.Sender}); err != nil {
			log.Printf("Error showing marketplace: %v", err)
		}
	})

	b.teleBot.Handle(&telebot.InlineButton{Unique: btnHelp}, func(c *telebot.Callback) {
		b.teleBot.Respond(c, &telebot.CallbackResponse{})
		b.showHelp(&telebot.Message{Sender: c.Sender})
	})

	// Register command handlers
	b.teleBot.Handle("/start", func(m *telebot.Message) {
		if err := b.registerUser(m); err != nil {
			log.Printf("Error registering user: %v", err)
		}
	})

	b.teleBot.Handle("/sell", func(m *telebot.Message) {
		args := strings.Fields(m.Text)
		if len(args) != 3 {
			b.showCreateOfferForm(m)
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
	
	b.teleBot.Handle("/marketplace", func(m *telebot.Message) {
		if err := b.showMarketplace(m); err != nil {
			log.Printf("Error showing marketplace: %v", err)
		}
	})
	
	b.teleBot.Handle("/help", func(m *telebot.Message) {
		b.showHelp(m)
	})
	
	// Handle unknown commands
	b.teleBot.Handle(telebot.OnText, func(m *telebot.Message) {
		// If message doesn't start with a command, show the main menu
		if !strings.HasPrefix(m.Text, "/") {
			b.sendMainMenu(m)
		}
	})

	log.Println("Bot started and ready to accept commands...")
	b.teleBot.Start()
} 
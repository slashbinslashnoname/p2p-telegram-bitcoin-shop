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
	
	// Callback prefixes
	cbConfirmPayment = "confirm_payment:"
	cbCancelOffer    = "cancel_offer:"
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

	// Send header message
	b.teleBot.Send(m.Sender, "📋 *Your offers:*", telebot.ModeMarkdown)
	
	// Create a menu for each offer
	for i, o := range offers {
		// Check if the offer is already completed or cancelled
		if o.Status == models.StatusCompleted || o.Status == models.StatusCancelled {
			continue // Skip completed or cancelled offers
		}
		
		// Check payment status if the offer is still pending
		isPaid := false
		if o.Status == models.StatusPending {
			paid, err := b.btcpay.CheckInvoiceStatus(o.InvoiceID)
			if err != nil {
				log.Printf("Failed to check invoice status for offer %d: %v", o.ID, err)
			}
			
			// If the invoice is paid but the status is still pending, update it
			if paid && o.Status == models.StatusPending {
				if err := b.database.UpdateOfferStatus(o.ID, models.StatusPaid); err != nil {
					log.Printf("Failed to update offer status: %v", err)
				} else {
					o.Status = models.StatusPaid
				}
				isPaid = true
			}
		} else if o.Status == models.StatusPaid {
			isPaid = true
		}
		
		// Determine status emoji
		statusEmoji := "⏳"
		if o.Status == models.StatusPaid {
			statusEmoji = "💰"
		} else if o.Status == models.StatusCompleted {
			statusEmoji = "✅"
		} else if o.Status == models.StatusCancelled {
			statusEmoji = "❌"
		}
		
		// Format the offer details
		offerDetails := fmt.Sprintf(
			"*Offer #%d*\n"+
			"🔹 Amount: %f BTC\n"+
			"🔹 Price: $%f\n"+
			"🔹 Date: %s\n"+
			"🔹 Status: %s %s\n",
			o.ID, o.AmountBTC, o.PriceUSD, o.CreatedAt.Format(time.RFC822), statusEmoji, o.Status)
		
		// Create buttons based on offer status
		menu := &telebot.ReplyMarkup{}
		var buttons []telebot.InlineButton
		
		// View invoice button
		btnViewInvoice := telebot.InlineButton{
			Text: "View Invoice",
			URL:  o.InvoiceLink,
		}
		buttons = append(buttons, btnViewInvoice)
		
		// If the offer is paid, add confirm payment button
		if isPaid {
			btnConfirmPayment := telebot.InlineButton{
				Text:   "✅ Confirm Payment Received",
				Unique: fmt.Sprintf("%s%d", cbConfirmPayment, o.ID),
			}
			buttons = append(buttons, btnConfirmPayment)
		}
		
		// Cancel offer button
		if o.Status == models.StatusPending {
			btnCancelOffer := telebot.InlineButton{
				Text:   "❌ Cancel Offer",
				Unique: fmt.Sprintf("%s%d", cbCancelOffer, o.ID),
			}
			buttons = append(buttons, btnCancelOffer)
		}
		
		// Add buttons to the menu
		menu.InlineKeyboard = [][]telebot.InlineButton{buttons}
		
		// Send each offer as a separate message with its own buttons
		if i < 10 { // Limit to 10 offers to avoid Telegram API limits
			b.teleBot.Send(m.Sender, offerDetails, menu, telebot.ModeMarkdown)
		}
	}
	
	// If there are more than 10 offers, send a summary message
	if len(offers) > 10 {
		b.teleBot.Send(m.Sender, fmt.Sprintf("Showing buttons for the first 10 offers. You have a total of %d offers.", len(offers)))
	}
	
	return nil
}

// confirmPayment confirms that payment has been received for an offer
func (b *Bot) confirmPayment(c *telebot.Callback) error {
	// Extract offer ID from callback data
	idStr := strings.TrimPrefix(c.Data, cbConfirmPayment)
	offerID, err := strconv.Atoi(idStr)
	if err != nil {
		return fmt.Errorf("invalid offer ID: %v", err)
	}
	
	// Get the offer
	offer, err := b.database.GetOffer(offerID)
	if err != nil {
		return fmt.Errorf("failed to get offer: %v", err)
	}
	
	// Check if the user is the owner of the offer
	if offer.UserID != c.Sender.ID {
		b.teleBot.Respond(c, &telebot.CallbackResponse{
			Text:      "You are not authorized to confirm this payment",
			ShowAlert: true,
		})
		return fmt.Errorf("unauthorized attempt to confirm payment for offer %d by user %d", offerID, c.Sender.ID)
	}
	
	// Check if the offer is in the correct status
	if offer.Status != models.StatusPaid {
		b.teleBot.Respond(c, &telebot.CallbackResponse{
			Text:      "This offer is not in the paid status",
			ShowAlert: true,
		})
		return fmt.Errorf("attempt to confirm payment for offer %d with status %s", offerID, offer.Status)
	}
	
	// Update the offer status
	if err := b.database.UpdateOfferStatus(offerID, models.StatusCompleted); err != nil {
		b.teleBot.Respond(c, &telebot.CallbackResponse{
			Text:      "Failed to update offer status",
			ShowAlert: true,
		})
		return fmt.Errorf("failed to update offer status: %v", err)
	}
	
	// Respond to the callback
	b.teleBot.Respond(c, &telebot.CallbackResponse{
		Text: "Payment confirmed! Funds have been released.",
	})
	
	// Send a confirmation message
	confirmMsg := fmt.Sprintf("✅ *Payment Confirmed*\n\nYou have confirmed receipt of payment for Offer #%d.\nThe transaction is now complete and funds have been released.", offerID)
	b.teleBot.Send(c.Sender, confirmMsg, telebot.ModeMarkdown)
	
	return nil
}

// cancelOffer cancels an offer
func (b *Bot) cancelOffer(c *telebot.Callback) error {
	// Extract offer ID from callback data
	idStr := strings.TrimPrefix(c.Data, cbCancelOffer)
	offerID, err := strconv.Atoi(idStr)
	if err != nil {
		return fmt.Errorf("invalid offer ID: %v", err)
	}
	
	// Get the offer
	offer, err := b.database.GetOffer(offerID)
	if err != nil {
		return fmt.Errorf("failed to get offer: %v", err)
	}
	
	// Check if the user is the owner of the offer
	if offer.UserID != c.Sender.ID {
		b.teleBot.Respond(c, &telebot.CallbackResponse{
			Text:      "You are not authorized to cancel this offer",
			ShowAlert: true,
		})
		return fmt.Errorf("unauthorized attempt to cancel offer %d by user %d", offerID, c.Sender.ID)
	}
	
	// Check if the offer is in the correct status
	if offer.Status != models.StatusPending {
		b.teleBot.Respond(c, &telebot.CallbackResponse{
			Text:      "Only pending offers can be cancelled",
			ShowAlert: true,
		})
		return fmt.Errorf("attempt to cancel offer %d with status %s", offerID, offer.Status)
	}
	
	// Update the offer status
	if err := b.database.UpdateOfferStatus(offerID, models.StatusCancelled); err != nil {
		b.teleBot.Respond(c, &telebot.CallbackResponse{
			Text:      "Failed to cancel offer",
			ShowAlert: true,
		})
		return fmt.Errorf("failed to update offer status: %v", err)
	}
	
	// Respond to the callback
	b.teleBot.Respond(c, &telebot.CallbackResponse{
		Text: "Offer cancelled successfully.",
	})
	
	// Send a confirmation message
	cancelMsg := fmt.Sprintf("❌ *Offer Cancelled*\n\nYou have cancelled Offer #%d.", offerID)
	b.teleBot.Send(c.Sender, cancelMsg, telebot.ModeMarkdown)
	
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
		// Only include pending offers in the marketplace
		if o.Status == models.StatusPending {
			sellerOffers[o.UserID] = append(sellerOffers[o.UserID], o)
		}
	}
	
	// If no pending offers, show a message
	if len(sellerOffers) == 0 {
		b.teleBot.Send(m.Sender, "No active offers available in the marketplace right now.")
		return nil
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
5. When you receive payment, confirm it to release funds

*Offer Status:*
⏳ Pending - Waiting for payment
💰 Paid - Payment received but not confirmed
✅ Completed - Payment confirmed, funds released
❌ Cancelled - Offer cancelled

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
	
	// Register handlers for confirm payment and cancel offer callbacks
	b.teleBot.Handle(telebot.OnCallback, func(c *telebot.Callback) {
		if strings.HasPrefix(c.Data, cbConfirmPayment) {
			if err := b.confirmPayment(c); err != nil {
				log.Printf("Error confirming payment: %v", err)
			}
		} else if strings.HasPrefix(c.Data, cbCancelOffer) {
			if err := b.cancelOffer(c); err != nil {
				log.Printf("Error cancelling offer: %v", err)
			}
		}
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
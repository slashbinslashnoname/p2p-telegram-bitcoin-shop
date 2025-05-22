# P2P Telegram Bitcoin Shop

A Telegram bot that allows users to sell Bitcoin via Lightning Network using BTCPay Server.

## Features

- User registration
- Create Bitcoin sell offers with Lightning Network invoices
- List and check status of your offers
- Integration with BTCPay Server for Lightning Network payments

## Project Structure

```
.
├── bot/            # Telegram bot implementation
├── btcpay/         # BTCPay Server API client
├── config/         # Configuration management
├── db/             # Database operations
├── models/         # Data models
├── main.go         # Application entry point
├── go.mod          # Go module file
├── go.sum          # Go dependencies checksum
├── .env            # Environment variables (create this file)
└── README.md       # This file
```

## Configuration

Create a `.env` file in the root directory with the following variables:

```
# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN=your_telegram_bot_token

# BTCPay Server Configuration
BTCPAY_URL=https://your.btcpayserver.com
BTCPAY_API_KEY=your_btcpay_api_key
BTCPAY_STORE_ID=your_btcpay_store_id

# Database Configuration
DB_PATH=./btc_trades.db
```

The application will automatically load these environment variables when it starts.

## Building and Running

```bash
# Build the application
go build -o btc-shop

# Run the application
./btc-shop
```

## Bot Commands

- `/start` - Register as a user
- `/sell <amount_btc> <price_usd>` - Create a sell offer
- `/list` - List your offers

## License

Jobware license - feel free to use it as you want. If you deploy and make money, hire me!

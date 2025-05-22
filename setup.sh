#!/bin/bash

# Setup script for P2P Telegram Bitcoin Shop

echo "Setting up P2P Telegram Bitcoin Shop..."
echo

# Check if .env file already exists
if [ -f .env ]; then
    read -p ".env file already exists. Do you want to overwrite it? (y/n): " overwrite
    if [ "$overwrite" != "y" ]; then
        echo "Setup aborted."
        exit 0
    fi
fi

# Collect configuration information
echo "Please provide the following information:"
echo

read -p "Telegram Bot Token: " telegram_token
read -p "BTCPay Server URL: " btcpay_url
read -p "BTCPay API Key: " btcpay_api_key
read -p "BTCPay Store ID: " btcpay_store_id
read -p "Database Path (default: ./btc_trades.db): " db_path

# Use default value for DB path if not provided
if [ -z "$db_path" ]; then
    db_path="./btc_trades.db"
fi

# Create .env file
cat > .env << EOF
# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN=$telegram_token

# BTCPay Server Configuration
BTCPAY_URL=$btcpay_url
BTCPAY_API_KEY=$btcpay_api_key
BTCPAY_STORE_ID=$btcpay_store_id

# Database Configuration
DB_PATH=$db_path
EOF

echo
echo ".env file has been created successfully!"
echo "You can now build and run the application." 
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/slashbinslashnoname/p2p-telegram-bitcoin-shop/models"
)

// Database wraps the SQL database connection
type Database struct {
	db *sql.DB
}

// NewDatabase initializes the database connection and schema
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Initialize database schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			user_id INTEGER PRIMARY KEY,
			username TEXT,
			created_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS offers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			amount_btc REAL,
			price_usd REAL,
			invoice_id TEXT,
			invoice_link TEXT,
			created_at TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(user_id)
		);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema: %v", err)
	}

	return &Database{db: db}, nil
}

// RegisterUser registers a new user in the database
func (d *Database) RegisterUser(userID int64, username string) error {
	_, err := d.db.Exec(
		"INSERT OR IGNORE INTO users (user_id, username, created_at) VALUES (?, ?, ?)",
		userID, username, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to register user: %v", err)
	}
	return nil
}

// UserExists checks if a user exists in the database
func (d *Database) UserExists(userID int64) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM users WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %v", err)
	}
	return count > 0, nil
}

// CreateOffer creates a new offer in the database
func (d *Database) CreateOffer(userID int64, amountBTC, priceUSD float64, invoiceID, invoiceLink string) error {
	_, err := d.db.Exec(
		"INSERT INTO offers (user_id, amount_btc, price_usd, invoice_id, invoice_link, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		userID, amountBTC, priceUSD, invoiceID, invoiceLink, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to create offer: %v", err)
	}
	return nil
}

// GetUserOffers retrieves all offers for a specific user
func (d *Database) GetUserOffers(userID int64) ([]models.Offer, error) {
	rows, err := d.db.Query("SELECT id, user_id, amount_btc, price_usd, invoice_id, invoice_link, created_at FROM offers WHERE user_id = ?", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch offers: %v", err)
	}
	defer rows.Close()

	var offers []models.Offer
	for rows.Next() {
		var o models.Offer
		if err := rows.Scan(&o.ID, &o.UserID, &o.AmountBTC, &o.PriceUSD, &o.InvoiceID, &o.InvoiceLink, &o.CreatedAt); err != nil {
			continue
		}
		offers = append(offers, o)
	}

	return offers, nil
}

// GetAllOffers retrieves all offers from all users, with optional limit
func (d *Database) GetAllOffers(limit int) ([]models.Offer, error) {
	query := "SELECT o.id, o.user_id, u.username, o.amount_btc, o.price_usd, o.invoice_id, o.invoice_link, o.created_at FROM offers o JOIN users u ON o.user_id = u.user_id ORDER BY o.created_at DESC"
	
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all offers: %v", err)
	}
	defer rows.Close()

	var offers []models.Offer
	for rows.Next() {
		var o models.Offer
		if err := rows.Scan(&o.ID, &o.UserID, &o.Username, &o.AmountBTC, &o.PriceUSD, &o.InvoiceID, &o.InvoiceLink, &o.CreatedAt); err != nil {
			continue
		}
		offers = append(offers, o)
	}

	return offers, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
} 
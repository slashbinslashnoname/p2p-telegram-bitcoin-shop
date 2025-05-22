package models

import (
	"time"
)

// Offer represents a Bitcoin selling offer
type Offer struct {
	ID          int
	UserID      int64
	AmountBTC   float64
	PriceUSD    float64
	InvoiceID   string
	InvoiceLink string
	CreatedAt   time.Time
} 
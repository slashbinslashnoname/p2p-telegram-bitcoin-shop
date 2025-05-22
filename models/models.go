package models

import (
	"time"
)

// OfferStatus represents the status of an offer
type OfferStatus string

const (
	// StatusPending indicates an offer waiting for payment
	StatusPending OfferStatus = "pending"
	// StatusPaid indicates an offer that has been paid but not confirmed by seller
	StatusPaid OfferStatus = "paid"
	// StatusCompleted indicates an offer that has been paid and confirmed by seller
	StatusCompleted OfferStatus = "completed"
	// StatusCancelled indicates an offer that has been cancelled
	StatusCancelled OfferStatus = "cancelled"
)

// Offer represents a Bitcoin selling offer
type Offer struct {
	ID          int
	UserID      int64
	Username    string // Username of the offer creator
	AmountBTC   float64
	PriceUSD    float64
	InvoiceID   string
	InvoiceLink string
	Status      OfferStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
} 
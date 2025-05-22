package btcpay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client wraps the BTCPay Server API client
type Client struct {
	client   *http.Client
	baseURL  string
	apiKey   string
	storeID  string
}

// NewClient initializes a BTCPay Server client
func NewClient(baseURL, apiKey, storeID string) *Client {
	return &Client{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: baseURL,
		apiKey:  apiKey,
		storeID: storeID,
	}
}

// CreateInvoice creates a BTCPay Server Lightning invoice
func (bc *Client) CreateInvoice(amountSats int64, description string) (string, string, error) {
	url := fmt.Sprintf("%s/api/v1/stores/%s/invoices", bc.baseURL, bc.storeID)
	body := map[string]interface{}{
		"amount":   float64(amountSats) / 100_000_000, // Convert satoshis to BTC
		"currency": "BTC",
		"metadata": map[string]string{
			"orderId": description,
		},
		"checkout": map[string]interface{}{
			"paymentMethods": []string{"BTC-LightningNetwork"},
			"expirationMinutes": 60,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal invoice request: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", bc.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := bc.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %v", err)
	}

	invoiceID, ok := result["id"].(string)
	if !ok {
		return "", "", fmt.Errorf("invalid invoice ID in response")
	}

	// Fetch invoice to get Lightning payment details
	invoiceURL := fmt.Sprintf("%s/api/v1/stores/%s/invoices/%s", bc.baseURL, bc.storeID, invoiceID)
	req, err = http.NewRequest("GET", invoiceURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create invoice fetch request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", bc.apiKey))

	resp, err = bc.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch invoice: %v", err)
	}
	defer resp.Body.Close()

	var invoiceData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&invoiceData); err != nil {
		return "", "", fmt.Errorf("failed to decode invoice response: %v", err)
	}

	paymentMethods, ok := invoiceData["checkoutLink"].(string)
	if !ok {
		return "", "", fmt.Errorf("invalid checkout link in response")
	}

	return invoiceID, paymentMethods, nil
}

// CheckInvoiceStatus checks if a BTCPay Server invoice has been paid
func (bc *Client) CheckInvoiceStatus(invoiceID string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/stores/%s/invoices/%s", bc.baseURL, bc.storeID, invoiceID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", bc.apiKey))

	resp, err := bc.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %v", err)
	}

	status, ok := result["status"].(string)
	if !ok {
		return false, fmt.Errorf("invalid status in response")
	}

	return status == "Settled" || status == "Complete", nil
} 
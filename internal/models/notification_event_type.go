package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NotificationEventType represents the type of notification event
type NotificationEventType string

const (
	SaleInvoiceCreate        NotificationEventType = "sale.invoice.create"
	SaleInvoiceUpdate        NotificationEventType = "sale.invoice.update"
	SaleCreditNoteCreate     NotificationEventType = "sale.credit_note.create"
	SaleCreditNoteUpdate     NotificationEventType = "sale.credit_note.update"
	PurchaseInvoiceCreate    NotificationEventType = "purchase.invoice.create"
	PurchaseInvoiceUpdate    NotificationEventType = "purchase.invoice.update"
	PurchaseCreditNoteCreate NotificationEventType = "purchase.credit_note.create"
	PurchaseCreditNoteUpdate NotificationEventType = "purchase.credit_note.update"
)

// ParseNotificationEventType parses a string into a NotificationEventType
// Returns an error if the event type is unknown
func ParseNotificationEventType(name string) (NotificationEventType, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))

	switch normalized {
	case string(SaleInvoiceCreate):
		return SaleInvoiceCreate, nil
	case string(SaleInvoiceUpdate):
		return SaleInvoiceUpdate, nil
	case string(SaleCreditNoteCreate):
		return SaleCreditNoteCreate, nil
	case string(SaleCreditNoteUpdate):
		return SaleCreditNoteUpdate, nil
	case string(PurchaseInvoiceCreate):
		return PurchaseInvoiceCreate, nil
	case string(PurchaseInvoiceUpdate):
		return PurchaseInvoiceUpdate, nil
	case string(PurchaseCreditNoteCreate):
		return PurchaseCreditNoteCreate, nil
	case string(PurchaseCreditNoteUpdate):
		return PurchaseCreditNoteUpdate, nil
	default:
		return "", fmt.Errorf("unknown notification event type: %s", name)
	}
}

// UnmarshalJSON implements custom JSON unmarshaling for NotificationEventType
// This validates and normalizes the event type during JSON parsing
func (net *NotificationEventType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsed, err := ParseNotificationEventType(s)
	if err != nil {
		return err
	}

	*net = parsed
	return nil
}

package models

import (
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
	name = strings.ToLower(strings.TrimSpace(name))

	validTypes := []NotificationEventType{
		SaleInvoiceCreate,
		SaleInvoiceUpdate,
		SaleCreditNoteCreate,
		SaleCreditNoteUpdate,
		PurchaseInvoiceCreate,
		PurchaseInvoiceUpdate,
		PurchaseCreditNoteCreate,
		PurchaseCreditNoteUpdate,
	}

	for _, eventType := range validTypes {
		if string(eventType) == name {
			return eventType, nil
		}
	}

	return "", fmt.Errorf("unknown notification event type: %s", name)
}

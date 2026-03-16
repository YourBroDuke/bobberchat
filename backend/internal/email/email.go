package email

import "context"

// Sender is the provider-agnostic interface for sending emails.
// Implementations can be swapped without changing business logic.
type Sender interface {
	SendEmail(ctx context.Context, msg Message) error
}

// Message represents an email to be sent.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

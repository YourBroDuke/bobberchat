package console

import (
	"context"
	"log"

	"github.com/bobberchat/bobberchat/backend/internal/email"
)

type Sender struct{}

func New() *Sender {
	return &Sender{}
}

func (s *Sender) SendEmail(ctx context.Context, msg email.Message) error {
	_ = ctx
	log.Printf("console email: to=%s subject=%q text=%q html=%q", msg.To, msg.Subject, msg.Text, msg.HTML)
	return nil
}

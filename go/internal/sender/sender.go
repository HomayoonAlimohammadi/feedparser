package sender

import (
	"context"
)

// Sender is the interface for broadcasting messages.
type Sender interface {
	Send(ctx context.Context, msg string) error
}

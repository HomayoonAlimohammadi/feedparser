package sender

import (
	"context"
	"fmt"
	"log/slog"
)

// StdoutSender prints messages to stdout (useful for debugging).
type StdoutSender struct {
	logger *slog.Logger
}

// NewStdoutSender creates a new stdout sender.
func NewStdoutSender(logger *slog.Logger) *StdoutSender {
	return &StdoutSender{logger: logger}
}

// Send prints the message to stdout.
func (s *StdoutSender) Send(_ context.Context, msg string) error {
	fmt.Println()
	fmt.Println("####################")
	fmt.Println(msg)
	fmt.Println("####################")
	fmt.Println()
	return nil
}

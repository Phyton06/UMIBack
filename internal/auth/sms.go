package auth

import "log/slog"

// Sender registra el código en el log sin enviarlo realmente.
type Sender struct{}

func (Sender) Send(phone, code string) error {
	slog.Info("mock SMS", "phone", phone, "code", code)
	return nil
}

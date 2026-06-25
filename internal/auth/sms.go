package auth

import "log/slog"

// Sender envía códigos OTP por SMS.
type Sender interface {
	Send(phone, code string) error
}

// MockSender registra el código en el log sin enviarlo realmente.
type MockSender struct{}

func (MockSender) Send(phone, code string) error {
	slog.Info("mock SMS", "phone", phone, "code", code)
	return nil
}

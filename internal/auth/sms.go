package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// Sender envía un código OTP al teléfono dado.
type Sender interface {
	Send(phone, code string) error
}

// LogSender registra el código en el log sin enviarlo realmente.
// Satisface la interfaz Sender para uso en desarrollo.
type LogSender struct{}

func (LogSender) Send(phone, code string) error {
	slog.Info("mock SMS", "phone", phone, "code", code)
	return nil
}

// TwilioSender envía códigos OTP mediante la API REST de Twilio.
type TwilioSender struct {
	accountSID  string
	authToken   string
	phoneNumber string
	client      *http.Client
}

// NewTwilioSender crea un sender respaldado por Twilio.
func NewTwilioSender(accountSID, authToken, phoneNumber string) *TwilioSender {
	return &TwilioSender{
		accountSID:  accountSID,
		authToken:   authToken,
		phoneNumber: phoneNumber,
		client:      &http.Client{},
	}
}

// Send envía un SMS vía Twilio con el código de verificación.
func (s *TwilioSender) Send(phone, code string) error {
	e164, err := normalizeToE164(phone)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.accountSID)
	data := url.Values{}
	data.Set("To", e164)
	data.Set("From", s.phoneNumber)
	data.Set("Body", "Tu codigo de verificacion UMI: "+code)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("twilio request: %w", err)
	}
	auth := base64.StdEncoding.EncodeToString([]byte(s.accountSID + ":" + s.authToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio error %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("SMS sent via Twilio", "phone", e164[:6]+"***")
	return nil
}

// AWSSNSSender envía códigos OTP mediante AWS SNS.
type AWSSNSSender struct {
	client   *sns.Client
	senderID string
}

// NewAWSSNSSender crea un sender respaldado por AWS SNS.
// La región se detecta automáticamente desde el entorno (AWS_REGION).
func NewAWSSNSSender(ctx context.Context, senderID string) (*AWSSNSSender, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &AWSSNSSender{
		client:   sns.NewFromConfig(cfg),
		senderID: senderID,
	}, nil
}

// Send publica un mensaje SMS vía AWS SNS con el código de verificación.
func (s *AWSSNSSender) Send(phone, code string) error {
	e164, err := normalizeToE164(phone)
	if err != nil {
		return err
	}
	msg := "Tu código de verificación UMI: " + code
	input := &sns.PublishInput{
		Message:     aws.String(msg),
		PhoneNumber: aws.String(e164),
	}
	if s.senderID != "" {
		input.MessageAttributes = map[string]types.MessageAttributeValue{
			"AWS.SNS.SMS.SenderID": {
				DataType:    aws.String("String"),
				StringValue: aws.String(s.senderID),
			},
		}
	}
	result, err := s.client.Publish(context.Background(), input)
	if err != nil {
		return fmt.Errorf("sns publish: %w", err)
	}
	slog.Info("SMS sent via SNS", "messageId", *result.MessageId, "phone", e164[:6]+"***")
	return nil
}

// normalizeToE164 convierte un número de teléfono mexicano a formato E.164.
//
// Reglas:
//   - 10 dígitos → prefijo +52 (ej. "5511223344" → "+525511223344")
//   - 12 dígitos con prefijo 52 → prefijo + (ej. "525511223344" → "+525511223344")
//   - Cualquier otra longitud → error
func normalizeToE164(phone string) (string, error) {
	digits := regexp.MustCompile(`\D`).ReplaceAllString(phone, "")
	if len(digits) == 10 {
		return "+52" + digits, nil
	}
	if len(digits) == 12 && digits[:2] == "52" {
		return "+" + digits, nil
	}
	return "", fmt.Errorf("cannot normalize to E.164: %s", phone)
}

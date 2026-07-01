package auth

import (
	"context"
	"testing"
)

func TestLogSender_Send_ReturnsNil(t *testing.T) {
	sender := LogSender{}
	err := sender.Send("+525511111111", "123456")
	if err != nil {
		t.Errorf("LogSender.Send() returned error: %v", err)
	}
}

func TestNormalizeToE164(t *testing.T) {
	tests := []struct {
		name    string
		phone   string
		want    string
		wantErr bool
	}{
		{
			name:    "ten digit MX",
			phone:   "5511223344",
			want:    "+525511223344",
			wantErr: false,
		},
		{
			name:    "already with +52 prefix",
			phone:   "+525511223344",
			want:    "+525511223344",
			wantErr: false,
		},
		{
			name:    "12 digits with 52 prefix",
			phone:   "525511223344",
			want:    "+525511223344",
			wantErr: false,
		},
		{
			name:    "invalid length too short",
			phone:   "123",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid length 9 digits",
			phone:   "551122334",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeToE164(tt.phone)
			if tt.wantErr {
				if err == nil {
					t.Errorf("normalizeToE164(%q): expected error, got nil", tt.phone)
				}
				return
			}
			if err != nil {
				t.Errorf("normalizeToE164(%q): unexpected error: %v", tt.phone, err)
				return
			}
			if got != tt.want {
				t.Errorf("normalizeToE164(%q) = %q, want %q", tt.phone, got, tt.want)
			}
		})
	}
}

func TestNewAWSSNSSender_NoCredentials(t *testing.T) {
	sender, err := NewAWSSNSSender(context.Background(), "")
	if err == nil {
		// Without AWS credentials, config.LoadDefaultConfig may still
		// succeed (default chain falls through). Verify sender is non-nil.
		if sender == nil {
			t.Error("NewAWSSNSSender returned nil sender without error")
		}
		return
	}
	// Error is expected without credentials. Verify sender is nil.
	if sender != nil {
		t.Error("NewAWSSNSSender returned sender alongside error")
	}
}

package auth

import (
	"testing"
)

func TestSender_Send_RetornaNil(t *testing.T) {
	sender := Sender{}
	err := sender.Send("+525511111111", "123456")
	if err != nil {
		t.Errorf("Sender.Send() retornó error: %v", err)
	}
}

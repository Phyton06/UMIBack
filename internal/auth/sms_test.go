package auth

import (
	"testing"
)

func TestMockSender_Send_RetornaNil(t *testing.T) {
	sender := MockSender{}
	err := sender.Send("+525511111111", "123456")
	if err != nil {
		t.Errorf("MockSender.Send() retornó error: %v", err)
	}
}

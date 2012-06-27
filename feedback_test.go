package apns

import (
	"testing"
)


func Test_parseAppleFeedbackMessage(t *testing.T) {

	msg, err := parseAppleFeedbackMessage([]byte{})
	if err == nil {
		t.Error("Invalid message passed: empty")
	}

	msg, err = parseAppleFeedbackMessage([]byte{0x0})
	if err == nil {
		t.Error("Invalid message passed: less than six bytes")
	}

	msg, err = parseAppleFeedbackMessage([]byte{0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 'A', 'B', 'C'})
	if err == nil {
		t.Error("Invalid message passed: Message Size is bigger than buffer")
	}

	msg, err = parseAppleFeedbackMessage([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x3, 0xA, 0xB, 0xC})
	if err != nil {
		t.Error(err)
	} else {
		if msg.DeviceToken != "0a0b0c" {
			t.Errorf("Invalid token found: %s", msg.DeviceToken)
		}
	}

}

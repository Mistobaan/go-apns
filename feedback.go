package apns

import (
	"encoding/hex"
	"encoding/binary"
	"errors"
	"bytes"
	"time"
	"log"
	"io"
	"bufio"
)


const APPLE_FEEDBACK string = "feedback.push.apple.com:2196"
const APPLE_FEEDBACK_SANDBOX string = "feedback.sandbox.push.apple.com:2196"

// NewFeedbackClient create a client for apple's Feedback system
//  
func NewFeedbackClient(endpoint, certificate, key string) (*ApnsConn, error) {
	return NewClient(endpoint, certificate, key)
}


type ApnsFeedbackMessage struct {
	Time_t      int32
	DeviceToken string
}

func parseAppleFeedbackMessage(readb []byte) (*ApnsFeedbackMessage, error) {
	var size int16
	var err error
	msg := &ApnsFeedbackMessage{}

	if len(readb) < 6 {
		return nil, errors.New("Not enough data in slice")
	}

	r := bytes.NewReader(readb)

	err = binary.Read(r, binary.BigEndian, &msg.Time_t)
	if err != nil {
		return nil, err
	}

	err = binary.Read(r, binary.BigEndian, &size)
	if err != nil {
		return nil, err
	}

	if (6 + int(size)) > len(readb) {
		return nil, errors.New("The Message size for the DeviceToken is bigger than the given buffer")
	}

	msg.DeviceToken = hex.EncodeToString(readb[6 : 6+int(size)])

	return msg, nil
}

// StartListening listens on a apple Feedback connection and produces an ApnsFeedbackMessage 
// each time a valid message is found
// If EOF is received the goroutine will try to re-connect 3 times waiting 5, 10 and 15 seconds
func (client *ApnsConn) StartListening() <-chan *ApnsFeedbackMessage {
	outChan := make(chan *ApnsFeedbackMessage)

	err := client.connect()
	if err != nil {
		panic(err)
	}

	go func() {

		readb := [4 + 2 + 32]byte{} // SSL default datapacket size

		client.tlsconn.SetReadDeadline(time.Time{}) //Do not timeout

		buff_reader := bufio.NewReader(client.tlsconn)

		for {
			n, err := buff_reader.Read(readb[:])
			if err == io.EOF {
				for count := 0; count < 3; count += 1 {
					err = client.shutdown()
					if err != nil {
						log.Printf("Error closing the connection: %v", err)
					}

					log.Printf("Feedback: try reconnection in 30 sec")

					time.Sleep(time.Second * 30)
					err = client.connect()
					if err != nil {
						log.Print(err)
					} else {
						log.Printf("Feedback: reconnected")
						client.tlsconn.SetReadDeadline(time.Time{}) //Do not timeout
						buff_reader = bufio.NewReader(client.tlsconn)
						break
					}
					if count == 3 {
						panic("Failed reconnecting more than 3 times to the feedback service")
					}
				}
			} else if err != nil {
				close(outChan)
				panic(err)
			} else {
				// parse all the messages
				msg, err := parseAppleFeedbackMessage(readb[:n])
				if err != nil {
					close(outChan)
					panic(err)
				} else {
					outChan <- msg
				}
			}
		}
	}()

	return outChan
}

//Package APNS Apple Notification System
// Inspired 
// from http://bravenewmethod.wordpress.com/2011/02/25/apple-push-notifications-with-go-language/

package apns

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"
	"bytes"
	"encoding/hex"
	"encoding/binary"
	"sync"
)


type apnsConn struct {
	tlsconn     *tls.Conn
	tls_cfg     tls.Config
	endpoint    string
	ReadTimeout time.Duration
	mu          sync.Mutex // Protecting the Apns Channel
}


func (client *apnsConn) Connect() (err error) {
	// connect to the APNS and wrap socket to tls client
	conn, err := net.Dial("tcp", client.endpoint)
	if err != nil {
		return err
	}

	if client.tlsconn != nil {
		client.Close()
	}

	client.tlsconn = tls.Client(conn, &client.tls_cfg)

	return nil
}


func Client(endpoint, certificate, key string) (*apnsConn, error) {

	// load certificates and setup config
	cert, err := tls.LoadX509KeyPair(certificate, key)
	if err != nil {
		return nil, err
	}

	apnsConn := &apnsConn{
		tlsconn: nil,
		tls_cfg: tls.Config{
			Certificates: []tls.Certificate{cert}},
		endpoint:    endpoint,
		ReadTimeout: 50 * time.Millisecond,
	}

	return apnsConn, nil
}

func (client *apnsConn) Close() (err error) {
	err = client.tlsconn.Close()
	return err
}


func (client *apnsConn) AttemptConnection() (err error) {
	// Force handshake to verify successful authorization.
	// Handshake is handled otherwise automatically on first
	// Read/Write attempt
	err = client.tlsconn.Handshake()
	if err != nil {
		return
	}

	return nil
}


func bwrite(w io.Writer, values ...interface{}) (err error) {
	for _, v := range values {
		err := binary.Write(w, binary.BigEndian, v)
		if err != nil {
			return err
		}
	}
	return nil
}


func (client *apnsConn) SendPayload(token, payload []byte) (err error) {

	transactionId := uint32(1)

	// expiration time, 1 hour
	expirationTime := uint32(time.Now().In(time.UTC).Add(time.Duration(1) * time.Hour).Unix())

	// build the actual pdu
	buffer := bytes.NewBuffer([]byte{})

	// command
	err = bwrite(buffer, uint8(1),
		transactionId,
		expirationTime,
		uint16(len(token)),
		token,
		uint16(len(payload)),
		payload)

	if err != nil {
		return
	}

	pdu := buffer.Bytes()

	// write pdu
	_, err = client.tlsconn.Write(pdu)

	if err != nil {
		return
	}

	// wait for 1 seconds error pdu from the socket
	client.tlsconn.SetReadDeadline(time.Now().Add(client.ReadTimeout))

	readb := [6]byte{}

	n, err := client.tlsconn.Read(readb[:])
	if err != nil {
		// TODO: we couldn't read in time .. keep going
		return nil
	}

	if n > 0 {
		var msg string
		var status uint8 = uint8(readb[1])

		switch status {
		case 0:
			msg = "No errors encountered"
		case 1:
			msg = "Processing Errors"
		case 2:
			msg = "Missing Device Token"
		case 3:
			msg = "Missing Topic"
		case 4:
			msg = "Missing Payload"
		case 5:
			msg = "Invalid Token Size"
		case 6:
			msg = "Invalid Topic Size"
		case 7:
			msg = "Invalid Payload Size"
		case 8:
			msg = "Invalid Token"
		case 255:
			msg = "None (Unknown)"
		}

		if status != 0 {
			fmt.Printf("received: %s, %s\n", msg, hex.EncodeToString(readb[:n]))
		} else {
			fmt.Printf("OK %s\n", hex.EncodeToString(readb[:n]))
		}
	}
	return nil
}

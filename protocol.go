// Package apns provides primitived to communicate with the Apple Notification System.
// http://developer.apple.com/library/mac/#documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/Introduction/Introduction.html#//apple_ref/doc/uid/TP40008194-CH1-SW1

// Inspired 
// from http://bravenewmethod.wordpress.com/2011/02/25/apple-push-notifications-with-go-language/

package apns

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type ApnsConn struct {
	tlsconn          *tls.Conn
	tls_cfg          tls.Config
	endpoint         string
	ReadTimeout      time.Duration
	mu               sync.Mutex // Protecting the Apns Channel
	transactionId    uint32     // keep transaction
	MAX_PAYLOAD_SIZE int        // default to 256 as per Apple specifications (June 9 2012) 
	connected        bool
}

func (client *ApnsConn) connect() (err error) {
	if client.connected {
		return nil
	}

	if client.tlsconn != nil {
		client.shutdown()
	}

	conn, err := net.Dial("tcp", client.endpoint)

	if err != nil {
		return err
	}

	client.tlsconn = tls.Client(conn, &client.tls_cfg)

	err = client.tlsconn.Handshake()

	if err == nil {
		client.connected = true
	}

	return err
}

// NewClient creates a new apns connection. endpoint and certificate are paths
// to the X.509 files. 
func NewClient(endpoint, certificate, key string) (*ApnsConn, error) {

	// load certificates and setup config
	cert, err := tls.LoadX509KeyPair(certificate, key)
	if err != nil {
		return nil, err
	}

	apnsConn := &ApnsConn{
		tlsconn: nil,
		tls_cfg: tls.Config{
			Certificates: []tls.Certificate{cert}},
		endpoint:         endpoint,
		ReadTimeout:      150 * time.Millisecond,
		MAX_PAYLOAD_SIZE: 256,
		connected:        false,
	}

	return apnsConn, nil
}

func (client *ApnsConn) shutdown() (err error) {
	err = nil
	if client.tlsconn != nil {
		err = client.tlsconn.Close()
		client.connected = false
	}
	return
}

// utility function
func bwrite(w io.Writer, values ...interface{}) (err error) {
	for _, v := range values {
		err := binary.Write(w, binary.BigEndian, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func createCommandOnePacket(transactionId uint32, expiration time.Duration, token, payload []byte) ([]byte, error) {

	expirationTime := uint32(time.Now().In(time.UTC).Add(expiration).Unix())

	// build the actual pdu
	buffer := bytes.NewBuffer([]byte{})

	err := bwrite(buffer, uint8(1),
		transactionId,
		expirationTime,
		uint16(len(token)),
		token,
		uint16(len(payload)),
		payload)

	if err != nil {
		return nil, err
	}

	pdu := buffer.Bytes()

	return pdu, nil
}

func createCommandZeroPacket(transactionId uint32, expiration time.Duration, token, payload []byte) ([]byte, error) {

	// build the actual pdu
	buffer := bytes.NewBuffer([]byte{})

	err := bwrite(buffer, uint8(0),
		uint16(len(token)),
		token,
		uint16(len(payload)),
		payload)

	if err != nil {
		return nil, err
	}

	pdu := buffer.Bytes()

	return pdu, nil
}

var errText = map[uint8]string{
	0:   "No errors encountered",
	1:   "Processing Errors",
	2:   "Missing Device Token",
	3:   "Missing Topic",
	4:   "Missing Payload",
	5:   "Invalid Token Size",
	6:   "Invalid Topic Size",
	7:   "Invalid Payload Size",
	8:   "Invalid Token",
	255: "None (Unknown)",
}

// SendPayload message to the specified device. 
// The commands waits for a response for no more that client.ReadTimeout.
// The method uses the same connection. If the connection is closed it tries to reopen it at the next
// time. 
func (client *ApnsConn) SendPayload(token, payload []byte, expiration time.Duration) (err error) {

	if len(payload) > client.MAX_PAYLOAD_SIZE {
		return errors.New(fmt.Sprintf("The payload exceeds maximum allowed", client.MAX_PAYLOAD_SIZE))
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	defer func() {
		if err != nil {
			client.shutdown()
		}
	}()

	// try to connect
	err = client.connect()
	if err != nil {
		return err
	}

	client.transactionId++

	var pkt []byte

	pkt, err = createCommandOnePacket(client.transactionId, expiration, token, payload)
	if err != nil {
		return
	}

	_, err = client.tlsconn.Write(pkt)

	if err != nil {
		return
	}

	// wait for 1 seconds error pdu from the socket
	client.tlsconn.SetReadDeadline(time.Now().Add(client.ReadTimeout))

	readb := [6]byte{}

	n, err := client.tlsconn.Read(readb[:])

	if err != nil {
		if e2, ok := err.(net.Error); ok && e2.Timeout() {
			err = nil
			return
		} else {
			return err
		}
	}

	if n > 1 {
		var status uint8 = uint8(readb[1])

		switch status {
		case 0:
			// OK
		case 1, 2, 3, 4, 5, 6, 7, 8, 255:
			return errors.New(errText[status])
		default:
			return errors.New(fmt.Sprintf("Unknown error code %s ", hex.EncodeToString(readb[:n])))
		}
	}

	err = nil
	return
}

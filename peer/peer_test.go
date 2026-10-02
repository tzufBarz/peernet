package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
)

type HandshakeResult struct {
	Session Session
	Err     error
}

func newConnectedPeerPair(t *testing.T) (*Peer, *Peer) {
	public1, private1, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	identity1 := Identity{
		Private: private1,
		Public:  public1,
		PeerID:  sha256.Sum256(public1),
	}

	public2, private2, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	identity2 := Identity{
		Private: private2,
		Public:  public2,
		PeerID:  sha256.Sum256(public2),
	}

	conn1, conn2 := bufferedPipe(t)

	result1chan := make(chan HandshakeResult)
	result2chan := make(chan HandshakeResult)

	go asyncHandshake(result1chan, conn1, identity1)
	go asyncHandshake(result2chan, conn2, identity2)

	result1 := <-result1chan
	result2 := <-result2chan

	if result1.Err != nil {
		t.Fatalf("Handshake failed: %v", result1.Err)
	}
	if result2.Err != nil {
		t.Fatalf("Handshake failed: %v", result2.Err)
	}

	return &Peer{Conn: conn1, Session: result1.Session}, &Peer{Conn: conn2, Session: result2.Session}
}

func fakeValidate(id PeerID) bool {
	return true
}

func asyncHandshake(resultChan chan HandshakeResult, conn net.Conn, identity Identity) {
	session, _, err := handshake(conn, identity, 0, fakeValidate)
	if err != nil {
		resultChan <- HandshakeResult{Err: err}
		return
	}

	resultChan <- HandshakeResult{Session: session}
}

func TestMessageDecryptsCorrectly(t *testing.T) {
	peer1, peer2 := newConnectedPeerPair(t)

	payload := []byte("Hello, World!")

	peer2.WriteMessage(Message{Type: MessageText, Payload: payload})
	msg, err := peer1.ReadMessage()
	if err != nil {
		t.Fatalf("Error reading message: %v", err)
	}

	if !bytes.Equal(msg.Payload, payload) {
		t.Errorf("Incorrect message received: expected \"%s\", got \"%s\"", payload, msg.Payload)
	}
}

func TestCocurrentWritesDecryptCorrectly(t *testing.T) {
	peer1, peer2 := newConnectedPeerPair(t)

	count := 100

	for i := range count {
		go peer2.WriteMessage(Message{Type: MessageText, Payload: fmt.Appendf(nil, "message-%d", i)})
	}

	received := make(map[int]struct{})

	for range count {
		msg, err := peer1.ReadMessage()
		if err != nil {
			t.Fatalf("Error reading message: %v", err)
		}

		if !strings.HasPrefix(string(msg.Payload), "message-") {
			t.Errorf("Incorrect message received: expected \"message-n\", got \"%s\"", msg.Payload)
		}

		numStr := strings.TrimPrefix(string(msg.Payload), "message-")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			t.Errorf("Error converting to int")
			continue
		}
		_, ok := received[num]
		if ok {
			t.Errorf("Received duplicate message: %s", msg.Payload)
			continue
		}
		received[num] = struct{}{}
	}

	if len(received) < count {
		t.Errorf("Missed %d messages", count-len(received))
	}
}

func TestCorruptedCiphertextFailsSafely(t *testing.T) {
	peer1, peer2 := newConnectedPeerPair(t)

	payload := encrypt([]byte("Meaningless Message"), peer2.Session.SendAEAD, peer2.sendCounter)
	payload[16] = ^payload[16]

	writeMessage(peer2.Conn, Message{Type: MessageText, Payload: payload})

	_, err := peer1.ReadMessage()
	if err == nil {
		t.Errorf("Expected decryption error but got none")
	}
}

func bufferedPipe(t *testing.T) (net.Conn, net.Conn) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	var conn2 net.Conn
	errCh := make(chan error, 1)
	go func() {
		var err error
		conn2, err = listener.Accept()
		errCh <- err
	}()

	conn1, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("failed to accept: %v", err)
	}

	return conn1, conn2
}

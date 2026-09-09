package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type MessageType byte

const (
	MessageHandshake         MessageType = 0
	MessageHandshakeResponse MessageType = 1
	MessagePing              MessageType = 2
	MessagePong              MessageType = 3
	MessageText              MessageType = 4
)

type HandshakePayload struct {
	Nonce     Nonce
	PublicKey PublicKey
}

type Message struct {
	Type    MessageType
	Payload []byte
}

const MaxMessageSize = 1024 * 1024

func handshake(conn net.Conn, identity Identity) (PeerID, error) {
	nonce, err := generateNonce()
	if err != nil {
		return PeerID{}, err
	}

	payload := HandshakePayload{
		Nonce:     nonce,
		PublicKey: identity.public,
	}

	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, payload); err != nil {
		return PeerID{}, err
	}

	if err := writeMessage(conn, Message{
		Type:    MessageHandshake,
		Payload: buf.Bytes(),
	}); err != nil {
		return PeerID{}, err
	}

	msg, err := readMessage(conn)
	if err != nil {
		return PeerID{}, err
	}

	if msg.Type != MessageHandshake {
		return PeerID{}, fmt.Errorf("expected handshake message")
	}

	if len(msg.Payload) != len(buf.Bytes()) {
		return PeerID{}, fmt.Errorf("invalid handshake length: %d", len(msg.Payload))
	}

	rPayload := &HandshakePayload{}

	if err := binary.Read(bytes.NewBuffer(msg.Payload), binary.BigEndian, rPayload); err != nil {
		return PeerID{}, err
	}

	writeMessage(conn, Message{
		Type:    MessageHandshakeResponse,
		Payload: ed25519.Sign(identity.private[:], rPayload.Nonce[:]),
	})

	msg, err = readMessage(conn)
	if err != nil {
		return PeerID{}, err
	}

	if msg.Type != MessageHandshakeResponse {
		return PeerID{}, fmt.Errorf("expected handshake response message")
	}

	if len(msg.Payload) != SignatureLength {
		return PeerID{}, fmt.Errorf("invalid signature length: %d", len(msg.Payload))
	}

	if !ed25519.Verify(rPayload.PublicKey[:], nonce[:], msg.Payload) {
		return PeerID{}, fmt.Errorf("verification failed")
	}

	return PeerID(sha256.Sum256(rPayload.PublicKey[:])), nil
}

func writeMessage(w io.Writer, msg Message) error {
	length := uint32(len(msg.Payload))

	if err := binary.Write(w, binary.BigEndian, msg.Type); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}

	_, err := w.Write(msg.Payload)
	return err
}

func readMessage(r io.Reader) (Message, error) {
	var msg Message

	if err := binary.Read(r, binary.BigEndian, &msg.Type); err != nil {
		return msg, err
	}

	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return msg, err
	}

	if length > MaxMessageSize {
		return msg, fmt.Errorf("message too large: %d bytes", length)
	}

	msg.Payload = make([]byte, length)

	if _, err := io.ReadFull(r, msg.Payload); err != nil {
		return msg, err
	}

	return msg, nil
}

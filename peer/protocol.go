package main

import (
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"

	"golang.org/x/crypto/chacha20poly1305"
)

type MessageType byte

const (
	MessageHandshake         MessageType = 0
	MessageHandshakeResponse MessageType = 1
	MessagePing              MessageType = 2
	MessagePong              MessageType = 3
	MessageText              MessageType = 4
)

var encryptedTypes = map[MessageType]bool{MessageText: true}

type HandshakeState struct {
	Nonce   Nonce
	Private *ecdh.PrivateKey
	Public  *ecdh.PublicKey
}

type Session struct {
	PeerID      PeerID
	PublicKey   ed25519.PublicKey
	SendAEAD    cipher.AEAD
	ReceiveAEAD cipher.AEAD
}

type Message struct {
	Type    MessageType
	Payload []byte
}

const MaxMessageSize = 1024 * 1024

func handshake(conn net.Conn, identity Identity) (Session, error) {
	var session Session

	state, err := createHandshakeState()
	if err != nil {
		return Session{}, err
	}

	if err := writeMessage(conn, Message{
		Type:    MessageHandshake,
		Payload: slices.Concat(state.Nonce[:], identity.Public, state.Public.Bytes()),
	}); err != nil {
		return Session{}, err
	}

	msg, err := readMessage(conn)
	if err != nil {
		return Session{}, err
	}

	if msg.Type != MessageHandshake {
		return Session{}, fmt.Errorf("expected handshake message")
	}

	if len(msg.Payload) != len(Nonce{})+X25519KeySize+ed25519.PublicKeySize {
		return Session{}, fmt.Errorf("invalid handshake length: %d", len(msg.Payload))
	}

	rNonce := msg.Payload[:len(Nonce{})]
	rSPublic := msg.Payload[len(Nonce{}) : len(Nonce{})+ed25519.PublicKeySize]
	rEPublic := msg.Payload[len(Nonce{})+ed25519.PublicKeySize:]

	writeMessage(conn, Message{
		Type:    MessageHandshakeResponse,
		Payload: ed25519.Sign(identity.Private, slices.Concat(state.Nonce[:], identity.Public, state.Public.Bytes(), rNonce, rSPublic, rEPublic)),
	})

	msg, err = readMessage(conn)
	if err != nil {
		return Session{}, err
	}

	if msg.Type != MessageHandshakeResponse {
		return Session{}, fmt.Errorf("expected handshake response message")
	}

	if len(msg.Payload) != SignatureLength {
		return Session{}, fmt.Errorf("invalid signature length: %d", len(msg.Payload))
	}

	if !ed25519.Verify(rSPublic, slices.Concat(rNonce, rSPublic, rEPublic, state.Nonce[:], identity.Public, state.Public.Bytes()), msg.Payload) {
		return Session{}, fmt.Errorf("verification failed")
	}

	rSPublicKey, err := ecdh.X25519().NewPublicKey(rEPublic)
	if err != nil {
		return Session{}, err
	}

	secret, err := state.Private.ECDH(rSPublicKey)
	if err != nil {
		return Session{}, err
	}

	session.PublicKey = rSPublic
	session.PeerID = PeerID(sha256.Sum256(rSPublic))

	sendKey, err := hkdf.Key(sha256.New, secret, []byte{}, string(slices.Concat([]byte("to"), session.PeerID[:])), EncryptionKeyLength)
	if err != nil {
		return Session{}, err
	}
	session.SendAEAD, err = chacha20poly1305.NewX(sendKey)
	if err != nil {
		return Session{}, err
	}
	receiveKey, err := hkdf.Key(sha256.New, secret, []byte{}, string(slices.Concat([]byte("to"), identity.PeerID[:])), EncryptionKeyLength)
	if err != nil {
		return Session{}, err
	}
	session.ReceiveAEAD, err = chacha20poly1305.NewX(receiveKey)
	if err != nil {
		return Session{}, err
	}

	return session, nil
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

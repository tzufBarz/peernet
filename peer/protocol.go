package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type MessageType byte

const (
	MessagePeerID MessageType = 0
	MessagePing   MessageType = 1
	MessagePong   MessageType = 2
	MessageText   MessageType = 3
)

type Message struct {
	Type    MessageType
	Payload []byte
}

const MaxMessageSize = 1024 * 1024

func handshake(conn net.Conn) (PeerID, error) {
	if err := writeMessage(conn, Message{
		Type:    MessagePeerID,
		Payload: peerID[:],
	}); err != nil {
		return PeerID{}, err
	}

	msg, err := readMessage(conn)
	if err != nil {
		return PeerID{}, err
	}

	if msg.Type != MessagePeerID {
		return PeerID{}, fmt.Errorf("expected peer ID message")
	}

	if len(msg.Payload) != len(PeerID{}) {
		return PeerID{}, fmt.Errorf("invalid peer ID length: %d", len(msg.Payload))
	}

	return PeerID(msg.Payload), nil
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

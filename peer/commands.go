package main

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func executeCommand(text string) (bool, error) {
	parts := strings.SplitN(text, " ", 2)
	switch parts[0] {
	case "connect":
		if len(parts) < 2 {
			return false, fmt.Errorf("Usage: connect <address:port>")
		}
		return false, connectCommand(parts[1])
	case "disconnect":
		if len(parts) < 2 {
			return false, fmt.Errorf("Usage: disconnect <peer-id>")
		}
		return false, disconnectCommand(parts[1])
	case "ping":
		if len(parts) < 2 {
			return false, fmt.Errorf("Usage: ping <peer-id>")
		}
		return false, pingCommand(parts[1])
	case "send":
		if len(parts) < 2 {
			return false, fmt.Errorf("Usage: send <peer-id> <message>")
		}
		return false, sendCommand(parts[1])
	case "exit":
		return true, nil
	default:
		return false, fmt.Errorf("Invalid command")
	}
}

func connectCommand(args string) error {
	if err := connect(args); err != nil {
		return fmt.Errorf("Failed to connect to %s: %v", args, err)
	}
	return nil
}

func disconnectCommand(args string) error {
	id, err := parsePeerID(args)
	if err != nil {
		return err
	}
	return disconnect(id)
}

func pingCommand(args string) error {
	id, err := parsePeerID(args)
	if err != nil {
		return err
	}
	peer, err := getPeer(id)
	if err != nil {
		return fmt.Errorf("Failed to ping: %v", err)
	}
	return peer.WriteMessage(Message{
		Type:    MessagePing,
		Payload: []byte{},
	})
}

func sendCommand(args string) error {
	argsArr := strings.SplitN(args, " ", 2)
	if len(argsArr) < 2 {
		return fmt.Errorf("Usage: send <peer-id> <message>")
	}
	id, err := parsePeerID(argsArr[0])
	if err != nil {
		return err
	}
	peer, err := getPeer(id)
	if err != nil {
		return fmt.Errorf("Failed to send message: %v", err)
	}
	return peer.WriteMessage(Message{
		Type:    MessageText,
		Payload: []byte(argsArr[1]),
	})
}

func parsePeerID(hexID string) (PeerID, error) {
	bytesId, err := hex.DecodeString(hexID)
	if err != nil || len(bytesId) != len(PeerID{}) {
		return PeerID{}, fmt.Errorf("Invalid ID")
	}
	return PeerID(bytesId), nil
}

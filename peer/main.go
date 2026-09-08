package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

type PeerID [16]byte

type Peer struct {
	ID      PeerID
	Conn    net.Conn
	writeMu sync.Mutex
}

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

var (
	peerID  PeerID
	peers   = make(map[PeerID]*Peer)
	peersMu sync.RWMutex
)

func main() {
	port := flag.Int("p", 5000, "Port")

	flag.Parse()

	var err error
	peerID, err = generateID()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Peer ID: %x\n", peerID)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Listening on :%d\n", *port)

	go listenLoop(listener)

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("> ")

	for scanner.Scan() {
		text := scanner.Text()
		parts := strings.SplitN(text, " ", 2)
		switch parts[0] {
		case "connect":
			if len(parts) < 2 {
				fmt.Println("Usage: connect <address:port>")
			} else {
				address := parts[1]
				if err := connect(address); err != nil {
					fmt.Printf("Failed to connect to %s: %v\n", address, err)
				}
			}
		case "disconnect":
			if len(parts) < 2 {
				fmt.Println("Usage: disconnect <peer-id>")
			} else {
				bytesId, err := hex.DecodeString(parts[1])
				if err != nil || len(bytesId) != len(PeerID{}) {
					fmt.Println("Invalid ID")
					break
				}
				disconnect(PeerID(bytesId))
			}
		case "exit":
			return
		default:
			fmt.Println("Invalid command")
		}
		fmt.Print("\r> ")
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error encountered while reading: %v", err)
	}
}

func generateID() (PeerID, error) {
	var id PeerID

	if _, err := rand.Read(id[:]); err != nil {
		return PeerID{}, err
	}

	return id, nil
}

func listenLoop(listener net.Listener) {
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go handle(conn)
	}
}

func connect(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}

	go handle(conn)

	return nil
}

func disconnect(id PeerID) {
	peersMu.Lock()
	peer, exists := peers[id]
	if exists {
		delete(peers, id)
	}
	peersMu.Unlock()

	if exists {
		peer.Conn.Close()
	}
}

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

func handle(conn net.Conn) {
	defer conn.Close()

	id, err := handshake(conn)
	if err != nil {
		fmt.Printf("\rHandshake with %s failed: %v\n> ", conn.RemoteAddr(), err)
		return
	}

	peersMu.Lock()

	if _, exists := peers[id]; exists {
		peersMu.Unlock()
		return
	}

	peers[id] = &Peer{
		ID:   id,
		Conn: conn,
	}

	peersMu.Unlock()

	defer func() {
		peersMu.Lock()

		peer, exists := peers[id]
		if exists && peer.Conn == conn {
			delete(peers, id)
		}

		peersMu.Unlock()
	}()

	fmt.Printf("\rConnected to %x\n> ", id)

	for {
		msg, err := readMessage(conn)
		if err != nil {
			fmt.Printf("\rDisconnected from %x\n> ", id)
			return
		}
		if msg.Type == MessageText {
			fmt.Printf("\r[%s]: %s\n> ", id, msg.Payload)
		}
	}
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

func (peer *Peer) WriteMessage(msg Message) error {
	peer.writeMu.Lock()
	defer peer.writeMu.Unlock()
	return writeMessage(peer.Conn, msg)
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

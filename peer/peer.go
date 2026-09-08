package main

import (
	"crypto/rand"
	"fmt"
	"net"
	"sync"
)

type PeerID [16]byte

type Peer struct {
	ID      PeerID
	Conn    net.Conn
	writeMu sync.Mutex
}

var (
	peerID  PeerID
	peers   = make(map[PeerID]*Peer)
	peersMu sync.RWMutex
)

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

func disconnect(id PeerID) error {
	peersMu.Lock()
	peer, exists := peers[id]
	if exists {
		delete(peers, id)
	}
	peersMu.Unlock()

	if exists {
		return peer.Conn.Close()
	}

	return nil
}

func (peer *Peer) WriteMessage(msg Message) error {
	peer.writeMu.Lock()
	defer peer.writeMu.Unlock()
	return writeMessage(peer.Conn, msg)
}

func getPeer(id PeerID) (*Peer, error) {
	peersMu.RLock()
	defer peersMu.RUnlock()
	peer, exists := peers[id]
	if !exists {
		return nil, fmt.Errorf("unknown peer")
	}
	return peer, nil
}

func handle(conn net.Conn) {
	defer conn.Close()

	id, err := handshake(conn)
	if err != nil {
		fmt.Printf("\rHandshake with %s failed: %v\n> ", conn.RemoteAddr(), err)
		return
	}

	peer := &Peer{
		ID:   id,
		Conn: conn,
	}

	peersMu.Lock()

	if _, exists := peers[id]; exists {
		peersMu.Unlock()
		return
	}

	peers[id] = peer

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
		switch msg.Type {
		case MessageText:
			fmt.Printf("\r<%x> %s\n> ", id, msg.Payload)
		case MessagePing:
			peer.WriteMessage(Message{
				Type:    MessagePong,
				Payload: []byte{},
			})
		case MessagePong:
			fmt.Printf("\r[%x] Pong!\n> ", id)
		}
	}
}

package main

import (
	"fmt"
	"net"
	"sync"
)

type PeerID [32]byte

type Peer struct {
	ID      PeerID
	Conn    net.Conn
	writeMu sync.Mutex
}

type LocalPeer struct {
	identity Identity
	peers    map[PeerID]*Peer
	peersMu  sync.RWMutex
}

func (local *LocalPeer) listenLoop(listener net.Listener) {
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go local.handle(conn)
	}
}

func (local *LocalPeer) connect(address string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}

	go local.handle(conn)

	return nil
}

func (local *LocalPeer) disconnect(id PeerID) error {
	local.peersMu.Lock()
	peer, exists := local.peers[id]
	if exists {
		delete(local.peers, id)
	}
	local.peersMu.Unlock()

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

func (local *LocalPeer) getPeer(id PeerID) (*Peer, error) {
	local.peersMu.RLock()
	defer local.peersMu.RUnlock()
	peer, exists := local.peers[id]
	if !exists {
		return nil, fmt.Errorf("unknown peer")
	}
	return peer, nil
}

func (local *LocalPeer) handle(conn net.Conn) {
	defer conn.Close()

	id, err := handshake(conn, local.identity)
	if err != nil {
		fmt.Printf("\rHandshake with %s failed: %v\n> ", conn.RemoteAddr(), err)
		return
	}

	peer := &Peer{
		ID:   id,
		Conn: conn,
	}

	local.peersMu.Lock()

	if _, exists := local.peers[id]; exists {
		local.peersMu.Unlock()
		return
	}

	local.peers[id] = peer

	local.peersMu.Unlock()

	defer func() {
		local.peersMu.Lock()

		peer, exists := local.peers[id]
		if exists && peer.Conn == conn {
			delete(local.peers, id)
		}

		local.peersMu.Unlock()
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

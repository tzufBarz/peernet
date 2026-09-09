package main

import (
	"fmt"
	"net"
	"sync"
)

type PeerID [32]byte

type Peer struct {
	Conn           net.Conn
	WriteMu        sync.Mutex
	Session        Session
	sendCounter    uint64
	receiveCounter uint64
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
	peer.WriteMu.Lock()
	defer peer.WriteMu.Unlock()
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

	session, err := handshake(conn, local.identity)
	if err != nil {
		fmt.Printf("\rHandshake with %s failed: %v\n> ", conn.RemoteAddr(), err)
		return
	}

	peer := &Peer{
		Session: session,
		Conn:    conn,
	}

	local.peersMu.Lock()

	if _, exists := local.peers[session.PeerID]; exists {
		local.peersMu.Unlock()
		return
	}

	local.peers[session.PeerID] = peer

	local.peersMu.Unlock()

	defer func() {
		local.peersMu.Lock()

		peer, exists := local.peers[session.PeerID]
		if exists && peer.Conn == conn {
			delete(local.peers, session.PeerID)
		}

		local.peersMu.Unlock()
	}()

	fmt.Printf("\rConnected to %x\n> ", session.PeerID)

	for {
		msg, err := readMessage(conn)
		if err != nil {
			fmt.Printf("\rDisconnected from %x\n> ", session.PeerID)
			return
		}
		switch msg.Type {
		case MessageText:
			plaintext, err := decrypt(msg.Payload, session.ReceiveAEAD, peer.receiveCounter)
			if err != nil {
				fmt.Printf("Decryption failed: %v\n> ", err)
				break
			}
			fmt.Printf("\r<%x> %s\n> ", session.PeerID, plaintext)
			peer.receiveCounter++
		case MessagePing:
			peer.WriteMessage(Message{
				Type:    MessagePong,
				Payload: []byte{},
			})
		case MessagePong:
			fmt.Printf("\r[%x] Pong!\n> ", session.PeerID)
		}
	}
}

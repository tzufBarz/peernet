package main

import (
	"crypto/ed25519"
	"fmt"
	"net"
	"sync"
)

type PeerID [32]byte

type Allowlist map[[ed25519.PublicKeySize]byte]bool

type Peer struct {
	Conn           net.Conn
	WriteMu        sync.Mutex
	Session        Session
	sendCounter    uint64
	receiveCounter uint64
}

type LocalPeer struct {
	identity   Identity
	peers      map[PeerID]*Peer
	peersMu    sync.RWMutex
	peerStore  *PeerStore
	listenPort uint16
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

func (local *LocalPeer) connect(name string) error {
	record, ok := local.peerStore.GetByName(name)
	if !ok {
		return fmt.Errorf("unknown peer: %s", name)
	}

	conn, err := net.Dial("tcp", record.Address)
	if err != nil {
		return err
	}

	go local.handle(conn)

	return nil
}

func (local *LocalPeer) disconnect(name string) error {
	record, ok := local.peerStore.GetByName(name)
	if !ok {
		return fmt.Errorf("unknown peer: %s", name)
	}

	local.peersMu.Lock()
	peer, exists := local.peers[record.ID]
	if exists {
		delete(local.peers, record.ID)
	}
	local.peersMu.Unlock()

	if exists {
		return peer.Conn.Close()
	}

	return nil
}

func (peer *Peer) WriteMessage(msg Message) {
	peer.WriteMu.Lock()
	defer peer.WriteMu.Unlock()

	shouldEncrypt := encryptedTypes[msg.Type]

	if shouldEncrypt {
		msg.Payload = encrypt(msg.Payload, peer.Session.SendAEAD, peer.sendCounter)
	}

	if err := writeMessage(peer.Conn, msg); err != nil {
		peer.Conn.Close()
		return
	}

	if shouldEncrypt {
		peer.sendCounter++
	}
}

func (peer *Peer) ReadMessage() (Message, error) {
	msg, err := readMessage(peer.Conn)
	if err != nil {
		return Message{}, err
	}

	if encryptedTypes[msg.Type] {
		msg.Payload, err = decrypt(msg.Payload, peer.Session.ReceiveAEAD, peer.receiveCounter)
		if err != nil {
			return Message{}, fmt.Errorf("decryption failed: %v", err)
		}
		peer.receiveCounter++
	}

	return msg, nil
}

func (local *LocalPeer) getPeer(name string) (*Peer, error) {
	record, ok := local.peerStore.GetByName(name)
	if !ok {
		return nil, fmt.Errorf("unknown peer: %s", name)
	}

	local.peersMu.RLock()
	defer local.peersMu.RUnlock()
	peer, exists := local.peers[record.ID]
	if !exists {
		return nil, fmt.Errorf("unknown peer")
	}
	return peer, nil
}

func (local *LocalPeer) validate(id PeerID) bool {
	_, allowed := local.peerStore.GetByID(id)
	return allowed
}

func (local *LocalPeer) handle(conn net.Conn) {
	defer conn.Close()

	session, dialableAddr, err := handshake(conn, local.identity, local.listenPort, local.validate)
	if err != nil {
		fmt.Printf("\rHandshake with %s failed: %v\n> ", conn.RemoteAddr(), err)
		return
	}

	peer := &Peer{
		Session: session,
		Conn:    conn,
	}

	local.peerStore.UpdateAddress(session.PeerID, dialableAddr)

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

	fmt.Printf("\rConnected to %s (%s)\n> ", local.getPeerName(session.PeerID), conn.RemoteAddr())

	for {
		msg, err := peer.ReadMessage()
		if err != nil {
			fmt.Printf("\rDisconnected from %s (%s): %v\n> ", local.getPeerName(session.PeerID), conn.RemoteAddr(), err)
			return
		}

		switch msg.Type {
		case MessageText:
			fmt.Printf("\r<%s> %s\n> ", local.getPeerName(session.PeerID), msg.Payload)
		case MessagePing:
			peer.WriteMessage(Message{Type: MessagePong})
		case MessagePong:
			fmt.Printf("\r[%s] Pong!\n> ", local.getPeerName(session.PeerID))
		}
	}
}

func (list Allowlist) Contains(key ed25519.PublicKey) bool {
	var arr [ed25519.PublicKeySize]byte
	copy(arr[:], key)

	return list[arr]
}

func (local *LocalPeer) getPeerName(id PeerID) string {
	if info, ok := local.peerStore.GetByID(id); ok && info.Name != "" {
		return info.Name
	}

	return fmt.Sprintf("%x", id[:4])
}

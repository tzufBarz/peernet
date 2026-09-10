package main

import (
	"bufio"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
)

type PeerID [32]byte

type Allowlist map[[ed25519.PublicKeySize]byte]bool

type Peer struct {
	Conn           net.Conn
	WriteMu        sync.Mutex
	Session        Session
	sendCounter    atomic.Uint64
	receiveCounter atomic.Uint64
}

type LocalPeer struct {
	identity  Identity
	peers     map[PeerID]*Peer
	peersMu   sync.RWMutex
	allowlist Allowlist
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

	if !local.allowlist.Contains(session.PublicKey) {
		fmt.Printf("\rKey not allowed: %x\n> ", session.PublicKey)
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
			plaintext, err := decrypt(msg.Payload, session.ReceiveAEAD, peer.receiveCounter.Load())
			if err != nil {
				fmt.Printf("Decryption failed: %v\n> ", err)
				break
			}
			fmt.Printf("\r<%x> %s\n> ", session.PeerID, plaintext)
			peer.receiveCounter.Add(1)
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

func loadAllowlist(allowpath string) (Allowlist, error) {
	file, err := os.Open(allowpath)
	if err != nil {
		return nil, err
	}

	list := make(Allowlist)

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		bytes, err := hex.DecodeString(line)
		if err != nil {
			return nil, err
		}
		if len(bytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid public key length: %d", len(bytes))
		}

		var key [ed25519.PublicKeySize]byte
		copy(key[:], ed25519.PublicKey(bytes))
		list[key] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error encountered while reading allowlist: %v", err)
	}

	return list, nil
}

func (list Allowlist) Contains(key ed25519.PublicKey) bool {
	var arr [ed25519.PublicKeySize]byte
	copy(arr[:], key)

	return list[arr]
}

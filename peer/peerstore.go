package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"github.com/BurntSushi/toml"
)

type PeerRecord struct {
	Name      string
	PublicKey ed25519.PublicKey
	ID        PeerID
	Address   string
}

type PeerStore struct {
	mu     sync.RWMutex
	peers  map[PeerID]*PeerRecord
	byName map[string]PeerID
}

type rawPeerRecord struct {
	PublicKey string `toml:"pubkey"`
	Address   string `toml:"address"`
}

func loadPeerStore(filePath string) (*PeerStore, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, err
	}

	var rawPeers struct {
		Peers map[string]rawPeerRecord `toml:"peers"`
	}

	if _, err := toml.DecodeFile(filePath, &rawPeers); err != nil {
		return nil, err
	}

	store := &PeerStore{
		peers:  make(map[PeerID]*PeerRecord),
		byName: make(map[string]PeerID),
	}

	for name, raw := range rawPeers.Peers {
		pubKeyBytes, err := hex.DecodeString(raw.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("peer %s has invalid public key hex: %v", name, err)
		}

		record := &PeerRecord{
			Name:      name,
			PublicKey: ed25519.PublicKey(pubKeyBytes),
			ID:        sha256.Sum256(pubKeyBytes),
			Address:   raw.Address,
		}

		store.peers[record.ID] = record
		store.byName[name] = record.ID
	}

	return store, nil
}

func (store *PeerStore) GetByID(id PeerID) (*PeerRecord, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	record, ok := store.peers[id]
	return record, ok
}

func (store *PeerStore) GetByName(name string) (*PeerRecord, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	id, ok := store.byName[name]
	if !ok {
		return nil, false
	}
	return store.GetByID(id)
}

func (store *PeerStore) UpdateAddress(id PeerID, addr string) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if record, ok := store.peers[id]; ok {
		record.Address = addr
	}
}

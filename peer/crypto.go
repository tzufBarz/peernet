package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

type Identity struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	peerID  PeerID
}

func loadIdentity(path string) (*Identity, error) {
	identity := &Identity{}

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	p, _ := pem.Decode(file)

	private, err := x509.ParsePKCS8PrivateKey(p.Bytes)
	if err != nil {
		return nil, err
	}

	var ok bool
	identity.private, ok = private.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("incorrect key - Ed25519 required")
	}

	identity.public = identity.private.Public().(ed25519.PublicKey)
	identity.peerID = PeerID(sha256.Sum256(identity.public))

	return identity, nil
}

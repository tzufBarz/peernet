package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

type PrivateKey [ed25519.PrivateKeySize]byte
type PublicKey [ed25519.PublicKeySize]byte
type Nonce [32]byte

const SignatureLength = 64

type Identity struct {
	private PrivateKey
	public  PublicKey
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
	confirmedPrivate, ok := private.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("incorrect key - Ed25519 required")
	}

	identity.private = PrivateKey(confirmedPrivate)

	identity.public = PublicKey(confirmedPrivate.Public().(ed25519.PublicKey))
	identity.peerID = PeerID(sha256.Sum256(identity.public[:]))

	return identity, nil
}

func generateNonce() (Nonce, error) {
	var nonce Nonce

	if _, err := rand.Read(nonce[:]); err != nil {
		return Nonce{}, err
	}

	return nonce, nil
}

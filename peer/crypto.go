package main

import (
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"os"
)

type Nonce [32]byte

const (
	SignatureLength       = 64
	X25519KeySize         = 32
	EncryptionKeyLength   = 32
	EncryptionNonceLength = 24
)

type Identity struct {
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
	PeerID  PeerID
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

	identity.Private = ed25519.PrivateKey(confirmedPrivate)

	identity.Public = ed25519.PublicKey(confirmedPrivate.Public().(ed25519.PublicKey))
	identity.PeerID = PeerID(sha256.Sum256(identity.Public))

	return identity, nil
}

func createHandshakeState() (HandshakeState, error) {
	handshakeState := HandshakeState{}

	if _, err := rand.Read(handshakeState.Nonce[:]); err != nil {
		return HandshakeState{}, err
	}

	var err error
	handshakeState.Private, err = ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return HandshakeState{}, err
	}

	handshakeState.Public = handshakeState.Private.PublicKey()

	return handshakeState, nil
}

func encrypt(plaintext []byte, aead cipher.AEAD, sendCounter uint64) []byte {
	fmt.Println(sendCounter)
	nonce := make([]byte, EncryptionNonceLength)
	binary.BigEndian.PutUint64(nonce, sendCounter)

	dst := make([]byte, 0, len(plaintext)+aead.Overhead())
	ciphertext := aead.Seal(dst, nonce, plaintext, nil)

	return ciphertext
}

func decrypt(ciphertext []byte, aead cipher.AEAD, receiveCounter uint64) ([]byte, error) {
	fmt.Println(receiveCounter)
	nonce := make([]byte, EncryptionNonceLength)
	binary.BigEndian.PutUint64(nonce, receiveCounter)

	plaintext, err := aead.Open(ciphertext[:0], nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

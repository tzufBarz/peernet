package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	tea "charm.land/bubbletea/v2"
)

func main() {
	port := flag.Int("p", 5000, "Advertising port")
	signature := flag.String("i", "", "Ed25519 identity signature file path")
	peerstorePath := flag.String("ps", "", "Peerstore TOML file")

	flag.Parse()

	if *signature == "" {
		fmt.Println("Identity is required")
		flag.Usage()
		return
	}

	if *peerstorePath == "" {
		fmt.Println("Peer store file is required")
		flag.Usage()
		return
	}

	peerStore, err := loadPeerStore(*peerstorePath)
	if err != nil {
		log.Fatalf("Failed to load peerstore file: %v", err)
	}

	identity, err := loadIdentity(*signature)
	if err != nil {
		log.Fatalf("Failed to load identity: %v", err)
	}

	local := &LocalPeer{
		identity:   *identity,
		peers:      make(map[PeerID]*Peer),
		peerStore:  peerStore,
		listenPort: uint16(*port),
	}

	p := tea.NewProgram(initialModel(local))

	local.program = p

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", local.listenPort))
	if err != nil {
		panic(err)
	}

	go local.listenLoop(listener)

	go local.autoconnect()

	if _, err := p.Run(); err != nil {
		log.Fatalf("Failed to run program: %v", err)
	}
}

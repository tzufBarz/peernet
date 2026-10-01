package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
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

	fmt.Printf("Peer ID: %x\n", identity.PeerID)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Listening on :%d\n", *port)

	go local.listenLoop(listener)

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("> ")

	for scanner.Scan() {
		exit, err := executeCommand(local, scanner.Text())
		if err != nil {
			fmt.Println(err)
		}
		if exit {
			return
		}
		fmt.Print("\r> ")
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error encountered while reading: %v", err)
	}
}

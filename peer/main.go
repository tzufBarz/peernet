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
	allowpath := flag.String("a", "", "Public key allow list text file")

	flag.Parse()

	if *signature == "" {
		fmt.Println("Identity is required")
		flag.Usage()
		return
	}

	if *allowpath == "" {
		fmt.Println("Allow list is required")
		flag.Usage()
		return
	}

	allowlist, err := loadAllowlist(*allowpath)
	if err != nil {
		log.Fatalf("Failed to load allow list file: %v", err)
	}

	identity, err := loadIdentity(*signature)
	if err != nil {
		log.Fatalf("Failed to load identity: %v", err)
	}

	local := &LocalPeer{
		identity:  *identity,
		peers:     make(map[PeerID]*Peer),
		allowlist: allowlist,
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

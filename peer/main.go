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

	flag.Parse()

	if *signature == "" {
		fmt.Println("Identity is required")
		flag.Usage()
		return
	}

	identity, err := loadIdentity(*signature)
	if err != nil {
		log.Fatal(err)
	}

	local := &LocalPeer{
		identity: *identity,
		peers:    make(map[PeerID]*Peer),
	}

	fmt.Printf("Peer ID: %x\n", identity.peerID)

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

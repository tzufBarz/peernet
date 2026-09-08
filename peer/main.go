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
	port := flag.Int("p", 5000, "Port")

	flag.Parse()

	var err error
	peerID, err = generateID()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Peer ID: %x\n", peerID)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Listening on :%d\n", *port)

	go listenLoop(listener)

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("> ")

	for scanner.Scan() {
		exit, err := executeCommand(scanner.Text())
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

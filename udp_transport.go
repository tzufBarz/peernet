package main

import (
	"bytes"
	"fmt"
	"net"
)

type UDPTransport struct {
	conn    *net.UDPConn
	Inbound chan UDPEnvelope
}

type UDPEnvelope struct {
	Msg  Message
	From *net.UDPAddr
}

func (t *UDPTransport) ListenLoop() {
	for {
		buf := make([]byte, 2048)
		n, addr, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		msg, err := readMessage(bytes.NewReader(buf[:n]))

		t.Inbound <- UDPEnvelope{msg, addr}
	}
}

func (t *UDPTransport) SendTo(addr string, msg Message) error {
	var buf bytes.Buffer
	if err := writeMessage(&buf, msg); err != nil {
		return err
	}

	fmt.Println(buf.Bytes())

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	_, err = t.conn.WriteToUDP(buf.Bytes(), udpAddr)

	return err
}

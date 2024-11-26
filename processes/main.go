package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	messagePing = "ping\n" // Initial message
	messagePong = "pong\n" // Response to the message
)

func main() {
	cliId, err := strconv.Atoi(os.Getenv("CLI_ID"))
	if err != nil {
		fmt.Printf("Error getting CLI_ID: %v\n", err)
		os.Exit(1)
	}
	if cliId == 0 {
		fmt.Println("CLI_ID not specified")
		os.Exit(1)
	}
	// bullyNodes, err := strconv.Atoi(os.Getenv("BULLY_NODES"))
	// if err != nil {
	// 	fmt.Printf("Error getting BULLY_NODES: %v\n", err)
	// 	os.Exit(1)
	// }

	go RunUDPListener(8000)

	for {
		time.Sleep(2 * time.Second)
		fmt.Printf("Hello, i am process: %d\n", cliId)
	}
}

type MessageType1 int

const (
	MessageTypePing MessageType1 = iota
	MessageTypePong MessageType1 = 1
)

type Message struct {
	Type MessageType1
}

func ListenUDP(port int) (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func RunUDPListener(port int) error {
	fmt.Printf("Running UDP listener on port %d\n", port)
	conn, err := ListenUDP(port)
	if err != nil {
		return err
	}
	decoder := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)
	handleConnection(decoder, encoder)
	return nil
}

func handleConnection(decoder *gob.Decoder, encoder *gob.Encoder) {
	for {
		msg := new(Message)
		err := decoder.Decode(msg)
		if err != nil {
			fmt.Printf("Error decoding message: %v\n", err)
			continue
		}
		switch msg.Type {
		case MessageTypePing:
			if err := encoder.Encode(Message{Type: MessageTypePong}); err != nil {
				fmt.Printf("Error encoding message: %v\n", err)
			}
		}
	}
}

package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

const (
	listenPort  = ":34567" // UDP listening port
	messagePing = "ping"   // Initial message
	messagePong = "pong"   // Response to the message
)

func main() {
	cliId := os.Getenv("CLI_ID")
	if cliId == "" {
		fmt.Println("CLI_ID not specified")
		os.Exit(1)
	}

	listenerAddr, err := net.ResolveUDPAddr("udp", listenPort)
	if err != nil {
		fmt.Printf("Error resolving UDP address: %v\n", err)
		os.Exit(1)
	}

	listener, err := net.ListenUDP("udp", listenerAddr)
	if err != nil {
		fmt.Printf("Error listening on UDP: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	remoteAddr := fmt.Sprintf("processes-%s:34567", cliId)
	conn, err := net.Dial("udp", remoteAddr)
	if err != nil {
		fmt.Printf("Error connecting to the remote process: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if cliId == "1" {
		sendMessage(conn, messagePing)
	}

	buffer := make([]byte, 1024)
	for {
		n, addr, err := listener.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("Error reading from UDP: %v\n", err)
			continue
		}

		received := string(buffer[:n])
		fmt.Printf("Received: %s from %s\n", received, addr)

		var response string
		if received == messagePing {
			response = messagePong
		} else if received == messagePong {
			response = messagePing
		} else {
			continue
		}

		sendMessage(conn, response)
	}
}

func sendMessage(conn net.Conn, message string) {
	_, err := conn.Write([]byte(message))
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
	} else {
		fmt.Printf("Sent: %s\n", message)
	}
	time.Sleep(1 * time.Second)
}

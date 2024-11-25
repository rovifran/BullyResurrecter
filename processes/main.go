package main

import (
	udpcomms "bullyresurrecter/udp-comms/socket"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	messagePing = "ping" // Initial message
	messagePong = "pong" // Response to the message
)

func main() {
	cliId := os.Getenv("CLI_ID")
	if cliId == "" {
		fmt.Println("CLI_ID not specified")
		os.Exit(1)
	}
	listenPort, err := strconv.Atoi(os.Getenv("LISTEN_PORT"))
	if err != nil {
		fmt.Printf("Error getting LISTEN_PORT: %v\n", err)
		os.Exit(1)
	}

	peers := []string{}
	if cliId == "1" {
		peers = []string{"processes-1"}
	} else if cliId == "2" {
		peers = []string{"processes-2"}
	}

	socket, err := udpcomms.NewSocket(fmt.Sprintf("processes-%s", cliId), peers, listenPort)
	if err != nil {
		fmt.Printf("Error creating socket: %v\n", err)
		os.Exit(1)
	}

	if cliId == "1" {
		sendMessage(socket, messagePing, peers[0])
	}

	buffer := make([]byte, 1024)
	for {
		n, name, err := socket.Receive(buffer)
		if err != nil {
			fmt.Printf("Error reading from UDP: %v\n", err)
			return
		}

		received := string(buffer[:n])

		var response string
		if received == messagePing {
			response = messagePong
		} else if received == messagePong {
			response = messagePing
		} else {
			continue
		}

		sendMessage(socket, response, name)
	}
}

func sendMessage(socket *udpcomms.Socket, message string, peer string) {
	err := socket.Send([]byte(message), peer, func(m udpcomms.Message) {
		fmt.Printf("Se cayo el proceso al cual le estoy hablando")
	})
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
	} else {
		fmt.Printf("Sent: %s\n", message)
	}
	time.Sleep(2 * time.Second)
}

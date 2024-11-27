package main

import (
	"fmt"
	"os"
	"strconv"
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
	bullyNodes, err := strconv.Atoi(os.Getenv("BULLY_NODES"))
	if err != nil {
		fmt.Printf("Error getting BULLY_NODES: %v\n", err)
		os.Exit(1)
	}

	node := NewNode(cliId)

	node.CreateTopology(bullyNodes)
	node.Run()
}


type MessageType int

const (
	MessageTypePing MessageType = iota
	MessageTypePong
)

type Message struct {
	PeerId int
	Type   MessageType
	Seq    int
}

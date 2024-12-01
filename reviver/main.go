package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
	bullyNodes, err := strconv.Atoi(os.Getenv("CLI_TOPOLOGY_NODES"))
	if err != nil {
		fmt.Printf("Error getting CLI_TOPOLOGY_NODES: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	node := NewNode(cliId, ctx)
	go func() {
		<-ctx.Done()
		fmt.Println("Received interrupt signal, shutting down")
		node.Close()
	}()

	node.CreateTopology(bullyNodes)
	node.Run()
}

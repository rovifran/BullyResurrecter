package main

import (
	"bullyresurrecter/shared"
	"fmt"
	"os"
	"strconv"
	"time"
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

	go shared.RunUDPListener(8080)

	for {
		time.Sleep(2 * time.Second)
		fmt.Printf("Hello, i am process: %d\n", cliId)
	}
}

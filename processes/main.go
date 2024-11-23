package main

import (
	"fmt"
	"time"
)

func main() {
	for {
		fmt.Println("Hola como andas?")
		time.Sleep(2 * time.Second)
	}
}
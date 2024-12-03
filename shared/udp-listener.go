package shared

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
)

type MessageType1 int

type ResurrecterMessage struct {
	Message     string
	ProcessName string
}

const (
	MessageTypePing MessageType1 = iota
	MessageTypePong MessageType1 = 1
)

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
	gob.Register(ResurrecterMessage{})
	conn, err := ListenUDP(port)
	if err != nil {
		return err
	}
	handleConnection(conn)
	return nil
}

func handleConnection(conn *net.UDPConn) {

	buffer := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("Error reading from UDP: %v\n", err)
			continue
		}

		var msg ResurrecterMessage
		decoder := gob.NewDecoder(bytes.NewReader(buffer[:n]))
		if err := decoder.Decode(&msg); err != nil {
			fmt.Printf("Error decoding message: %v\n", err)
			continue
		}

		switch msg.Message {
		case "ping":
			response := ResurrecterMessage{
				Message:     "pong",
				ProcessName: msg.ProcessName,
			}

			var buf bytes.Buffer
			encoder := gob.NewEncoder(&buf)
			if err := encoder.Encode(response); err != nil {
				fmt.Printf("Error encoding message: %v\n", err)
				continue
			}

			_, err = conn.WriteToUDP(buf.Bytes(), remoteAddr)
			if err != nil {
				fmt.Printf("Error sending response: %v\n", err)
			}
		}
	}
}

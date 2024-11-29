package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"bullyresurrecter/shared"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Resurrecter struct {
	containerName string
	connAddr      *net.UDPAddr
	conn          *net.UDPConn
	stopChan      chan struct{}
}

func NewResurrecter(containerName string, stopChan chan struct{}) *Resurrecter {
	log.Printf("NewResurrecter for %s", containerName)
	connAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:8080", containerName))
	if err != nil {
		log.Fatalf("Error resolving UDP address: %v", err)
	}

	log.Printf("Dialing UDP to %s", connAddr)
	conn, err := net.DialUDP("udp", nil, connAddr)
	if err != nil {
		log.Fatalf("Error dialing UDP: %v", err)
	}

	return &Resurrecter{
		containerName: containerName,
		connAddr:      connAddr,
		conn:          conn,
		stopChan:      stopChan,
	}
}

func (r *Resurrecter) Start(port int) {
	gob.Register(shared.ResurrecterMessage{})
	connAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Error resolving UDP address: %v", err)
	}
	fmt.Printf("Listening on %s\n", connAddr)
	conn, err := net.ListenUDP("udp", connAddr)
	if err != nil {
		log.Fatalf("Error listening on UDP: %v", err)
	}
	r.conn = conn

	r.RunMainLoop()
}

func (r *Resurrecter) RunMainLoop() {
	for {
		select {
		case <-r.stopChan:
			return
		case <-time.After(shared.ResurrecterPingInterval):
			r.sendPing()
		}
	}
}

func (r *Resurrecter) sendPing() {
	message := shared.ResurrecterMessage{
		Message:     "ping",
		ProcessName: r.containerName,
	}

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(message); err != nil {
		log.Printf("Error encoding message: %v", err)
		return
	}

	retries := 0
	pingTimeout := shared.ResurrecterPingTimeout
	for retries < shared.ResurrecterPingRetries {
		log.Printf("Sending ping to %s", r.connAddr)
		_, err := r.conn.WriteToUDP(buf.Bytes(), r.connAddr)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		if err := r.conn.SetReadDeadline(time.Now().Add(pingTimeout)); err != nil {
			log.Printf("Error setting read deadline: %v", err)
			return
		}

		response := make([]byte, 1024)
		n, _, err := r.conn.ReadFromUDP(response)
		if err != nil && errors.Is(err, os.ErrDeadlineExceeded) {
			retries++
			pingTimeout *= 2	
			continue
		}

		var answer shared.ResurrecterMessage
		decoder := gob.NewDecoder(bytes.NewReader(response[:n]))
		if err := decoder.Decode(&answer); err != nil {
			log.Printf("Error decoding response: %v", err)
			continue
		}
		return
	}

	log.Printf("Container %s is dead", r.containerName)
	r.Resurrect()
}

func (r *Resurrecter) Resurrect() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error creating Docker client: %v", err)
		return
	}

	log.Printf("Restarting container: %s", r.containerName)
	if err := cli.ContainerStart(context.Background(), r.containerName, container.StartOptions{}); err != nil {
		log.Printf("Error restarting container: %v", err)
	}

	time.Sleep(shared.ResurrecterRestartDelay)
}

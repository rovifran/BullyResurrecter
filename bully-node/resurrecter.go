package main

import (
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

type ResurrecterMessage struct {
	Message     string
	processName string
}

type Resurrecter struct {
	containerName string
	connAddr      *net.UDPAddr
	conn          *net.UDPConn
	encoder       *gob.Encoder
	decoder       *gob.Decoder
	stopChan      chan struct{}
}

func NewResurrecter(containerName string, stopChan chan struct{}) *Resurrecter {
	connAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:8080", containerName))
	if err != nil {
		log.Fatalf("Error resolving UDP address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, connAddr)
	if err != nil {
		log.Fatalf("Error dialing UDP: %v", err)
	}

	return &Resurrecter{
		containerName: containerName,
		connAddr:      connAddr,
		conn:          conn,
		stopChan:      stopChan,
		encoder:       gob.NewEncoder(conn),
		decoder:       gob.NewDecoder(conn),
	}
}

func (r *Resurrecter) Start() {
	conn, err := net.ListenUDP("udp", r.connAddr)
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
	message := ResurrecterMessage{
		Message:     "ping",
		processName: r.containerName,
	}

	err := r.encoder.Encode(message)
	if err != nil {
		log.Printf("Error encoding message: %v", err)
		return
	}

	retries := 0
	pingTimeout := shared.ResurrecterPingTimeout
	for retries < shared.ResurrecterPingRetries {
		if err := r.conn.SetReadDeadline(time.Now().Add(pingTimeout)); err != nil {
			log.Printf("Error setting read deadline: %v", err)
			return
		}
		var answer ResurrecterMessage
		err := r.decoder.Decode(&answer)
		if err != nil && errors.Is(err, os.ErrDeadlineExceeded) {
			retries++
			pingTimeout *= 2
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

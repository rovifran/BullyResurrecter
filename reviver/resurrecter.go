package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"log"
	"net"
	"time"

	"bullyresurrecter/shared"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Resurrecter struct {
	processList [][]string
	conn        *net.UDPConn
	stopContext context.Context
}

func NewResurrecter(processList [][]string, stopContext context.Context) *Resurrecter {
	return &Resurrecter{
		processList: processList,
		stopContext: stopContext,
	}
}

func (r *Resurrecter) Close() {
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *Resurrecter) Start() {
	gob.Register(shared.ResurrecterMessage{})
	connAddr, err := net.ResolveUDPAddr("udp", ":8081")
	if err != nil {
		// log.Printf("Error resolving UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", connAddr)
	if err != nil {
		// log.Printf("Error listening on UDP: %v", err)
	}
	r.conn = conn

	responseMap := make(map[string]chan struct{})

	for _, process := range r.processList {
		processChan := make(chan struct{})
		go r.RunMainLoop(process, processChan)
		responseMap[process[0]] = processChan
	}

	for {
		if err := r.receiveResponses(responseMap); err != nil {
			r.Close()
			return
		}
	}
}

func (r *Resurrecter) receiveResponses(responseMap map[string]chan struct{}) error {
	for {
		response := make([]byte, 1024)
		n, _, err := r.conn.ReadFromUDP(response)
		if err != nil {
			return err
		}

		var message shared.ResurrecterMessage
		decoder := gob.NewDecoder(bytes.NewReader(response[:n]))
		if err := decoder.Decode(&message); err != nil {
			log.Printf("Error decoding response: %v", err)
			return err
		}

		responseChan, ok := responseMap[message.ProcessName]
		if !ok {
			log.Printf("Unknown process: %s", message.ProcessName)
			continue
		}

		responseChan <- struct{}{}
	}
}

func (r *Resurrecter) RunMainLoop(process []string, responseChan chan struct{}) {
	for {
		select {
		case <-r.stopContext.Done():
			return
		case <-time.After(shared.ResurrecterPingInterval):
			r.sendPing(process, responseChan)
		}
	}
}

func (r *Resurrecter) sendPing(process []string, responseChan chan struct{}) {
	if r.conn == nil {
		r.Resurrect(process[0])
		return
	}

	message := shared.ResurrecterMessage{
		Message:     "ping",
		ProcessName: process[0],
	}

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(message); err != nil {
		log.Printf("Error encoding message: %v", err)
		return
	}

	retries := 0
	pingTimeout := shared.ResurrecterPingTimeout

	for {
		_, err := r.conn.WriteToUDP(buf.Bytes(), &net.UDPAddr{IP: net.ParseIP(process[1]), Port: 8080})
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		select {
		case <-r.stopContext.Done():
			return
		case <-responseChan:
			return
		case <-time.After(pingTimeout):
			retries++
			pingTimeout *= 2

			if retries >= shared.ResurrecterPingRetries {
				log.Printf("Container %s is dead", process[0])
				r.Resurrect(process[0])
				return
			}
		}
	}
}

func (r *Resurrecter) Resurrect(process string) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Error creating Docker client: %v", err)
		return
	}

	log.Printf("Resurrecting container: %s", process)
	if err := cli.ContainerStart(context.Background(), process, container.StartOptions{}); err != nil {
		log.Printf("Error restarting container: %v", err)
	}

	time.Sleep(shared.ResurrecterRestartDelay)
}

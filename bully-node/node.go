package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"sync"
	"time"
)

type Node struct {
	id         int
	peers      []*Peer
	serverConn *net.TCPListener
	serverAddr *net.TCPAddr
	peerLock   sync.Mutex
}

func NewNode(id int) *Node {
	peers := []*Peer{}

	serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("10.5.1.%d:8000", id))
	if err != nil {
		fmt.Printf("Error resolving server address: %v\n", err)
		os.Exit(1)
	}

	return &Node{id: id, peers: peers, serverAddr: serverAddr, peerLock: sync.Mutex{}}
}

func (n *Node) Run() {
	fmt.Printf("Running node %d\n", n.id)
	go n.Listen() // solo responde por esta conn TCP, no arranca la topologia nunca (no PING, no ELECTION, etc)

	time.Sleep(1 * time.Second)

	start := time.Now()
	for i := 0; i < 100; i++ {

		for _, peer := range n.peers {
			peer.Send(Message{PeerId: n.id, Type: MessageTypePing, Seq: i}) // mando por mi llamado el PING

			response := new(Message)
			err := peer.decoder.Decode(response) // se que el proximo mensaje por este channel solo va a ser PONG porque no me va a mandar PING por aca
			if err != nil {
				fmt.Printf("Error decoding response: %v\n", err)
				continue
			}

			fmt.Printf("Received response from %s: %+v\n", peer.ip.String(), response)

			if rand.Int32N(100) < 3 && peer.conn != nil {
				log.Printf("Randomly closing peer %s\n", peer.ip.String())
				peer.Close()
			}
		}
	}

	log.Printf("Total time: %dms\n", time.Since(start).Milliseconds())

	select {}
}

func (n *Node) CreateTopology(bullyNodes int) {
	for i := 1; i <= bullyNodes; i++ {
		if i == n.id {
			continue
		}
		peerIp := fmt.Sprintf("10.5.1.%d", i)
		peer := NewPeer(&peerIp)
		if peer != nil {
			n.peers = append(n.peers, peer)
		}
	}
}

func (n *Node) Listen() {
	serverConn, err := net.ListenTCP("tcp", n.serverAddr)
	if err != nil {
		fmt.Printf("Error listening on server address: %v\n", err)
		os.Exit(1)
	}
	n.serverConn = serverConn

	for {

		conn, err := serverConn.AcceptTCP()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}

		go n.RespondToPeer(conn)
	}
}

func (n *Node) RespondToPeer(conn *net.TCPConn) {
	log.Printf("Peer %s connected\n", conn.RemoteAddr().String())

	// if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
	// 	fmt.Printf("Error setting read deadline: %v\n", err)
	// 	conn.Close()
	// 	return
	// }

	read := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)

	for {
		message := new(Message)
		err := read.Decode(message)

		if err != nil {
			fmt.Printf("Error decoding message: %v\n", err)
			conn.Close()
			return
		}

		switch message.Type {
		case MessageTypePing:
			encoder.Encode(Message{PeerId: n.id, Type: MessageTypePong, Seq: message.Seq})
		case MessageTypePong:
		}
	}
}

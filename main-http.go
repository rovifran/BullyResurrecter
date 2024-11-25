package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	messagePing = "ping\n" // Initial message
	messagePong = "pong\n" // Response to the message
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
	go node.Listen()

	time.Sleep(1 * time.Second)

	if cliId == 1 {
		start := time.Now()
		for i := 0; i < 1000; i++ {
			node.peers[0].Send(fmt.Sprintf("Peer: %d, Message: %d\n", cliId, i))
		}

		elapsed := time.Since(start)
		log.Printf("Time elapsed: %dms\n", elapsed.Milliseconds())
	}

	select {}
}

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)

		if err != nil {
			log.Printf("Error reading request body: %v\n", err)
			return
		}

		log.Printf("[%s] Received from %s message: %s\n", r.Proto, r.RemoteAddr, string(body))
	})

	http.ListenAndServe(n.serverAddr.String(), nil)
}

type Peer struct {
	ip *net.IP
}

func NewPeer(ip *string) *Peer {
	ipAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:8000", *ip))
	if err != nil {
		fmt.Printf("Error resolving peer IP address: %v\n", err)
		return nil
	}

	return &Peer{ip: &ipAddr.IP}
}

func (p *Peer) Send(message string) error {
	_, err := http.Post(fmt.Sprintf("http://%s:8000/", p.ip.String()), "text/plain", strings.NewReader(message))
	if err != nil {
		return err
	}
	return nil
}

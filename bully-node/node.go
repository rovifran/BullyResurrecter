package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type NodeState int

const (
	NodeStateFollower NodeState = iota
	NodeStateCandidate
	NodeStateWaitingCoordinator
	NodeStateCoordinator
)

const (
	ElectionTimeout   = 5 * time.Second
	OkResponseTimeout = 2 * time.Second
)

type Node struct {
	id            int
	peers         []*Peer
	serverConn    *net.TCPListener
	serverAddr    *net.TCPAddr
	peerLock      sync.Mutex
	state         NodeState
	currentLeader int
	stopChan      chan struct{}
}

const INACTIVE_LEADER = -1

func NewNode(id int) *Node {
	peers := []*Peer{}
	stopChan := make(chan struct{})

	serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("10.5.1.%d:8000", id))
	if err != nil {
		fmt.Printf("Error resolving server address: %v\n", err)
		os.Exit(1)
	}

	return &Node{
		id:           id,
		peers:        peers,
		serverAddr:   serverAddr,
		peerLock:     sync.Mutex{},
		state:        NodeStateFollower,
		currentLeader: INACTIVE_LEADER,
		stopChan:      stopChan,
	}
}

func (n *Node) Close() {
	if n.serverConn != nil {
		if err := n.serverConn.Close(); err != nil {
			fmt.Printf("Error closing server connection: %v\n", err)
		}
	}
}

func (n *Node) Run() {
	fmt.Printf("Running node %d\n", n.id)
	go n.Listen() // solo responde por esta conexión TCP, no arranca la topología nunca (no PING, no ELECTION, etc)
	go n.StartElection() // siempre queremos arrancar una eleccion cuando se levanta el nodo

	<-n.stopChan
}

func (n *Node) StartElection() {
	log.Printf("Node %d starting election\n", n.id)
	n.ChangeState(NodeStateCandidate)

	// Keep track of which peers we're waiting for responses from
	waitingResponses := make(map[int]bool)
	responseChan := make(chan int) // Channel to receive OK responses
	timeoutChan := time.After(ElectionTimeout)

	// Send election messages only to higher-numbered peers
	for _, peer := range n.peers {
		if peer.id > n.id {
			// Send election message
			if err := peer.Send(Message{
				PeerId: n.id,
				Type:   MessageTypeElection,
			}); err != nil {
				log.Printf("Failed to send election message to peer %d: %v", peer.id, err)
				continue
			}

			waitingResponses[peer.id] = true

			// Start a goroutine to wait for OK response from this peer
			go func(peerId int) {
				response := new(Message)
				if err := peer.conn.SetReadDeadline(time.Now().Add(OkResponseTimeout)); err != nil {
					log.Printf("Error setting read deadline for peer %d: %v", peerId, err)
					return
				}
				if err := peer.decoder.Decode(response); err != nil {
					log.Printf("Error decoding response from peer %d: %v", peerId, err)
					return
				}

				if response.Type == MessageTypeOk {
					responseChan <- peerId
				}
			}(peer.id)
		}
	}

	// If no higher-numbered peers, become leader immediately
	if len(waitingResponses) == 0 {
		n.BecomeLeader()
		return
	}

	// Wait for responses or timeout
	for len(waitingResponses) > 0 {
		select {
		case peerId := <-responseChan:
			delete(waitingResponses, peerId)
			log.Printf("Received OK from peer %d", peerId)
			n.ChangeState(NodeStateFollower)

		case <-timeoutChan:
			// If we timeout waiting for responses, we become the leader
			log.Printf("Election timeout reached, becoming leader")
			if n.GetState() == NodeStateCandidate {
				n.BecomeLeader()
			}
			return
		}
	}

	// If we got here, it means we received all OK responses
	// Wait for coordinator message from someone else
	log.Printf("Received all OK responses, waiting for coordinator message")
	n.WaitForCoordinator()
}

func (n *Node) WaitForCoordinator() {
	// Wait for coordinator message with timeout
	coordTimeout := time.After(ElectionTimeout + OkResponseTimeout)

	for {
		select {
		case <-coordTimeout:
			// If we timeout waiting for coordinator, start new election
			log.Printf("Timeout waiting for coordinator, starting new election")
			n.StartElection()
			return

			// You might want to add a channel here to receive coordinator messages
			// from the message handling routine
		}
	}
}

func (n *Node) BecomeLeader() {
	log.Printf("Node %d becoming leader\n", n.id)
	n.ChangeState(NodeStateCoordinator)
	n.SetCurrentLeader(n.id)

	// Send coordinator message to all peers
	for _, peer := range n.peers {
		if err := peer.Send(Message{PeerId: n.id, Type: MessageTypeCoordinator}); err != nil {
			log.Printf("Failed to send coordinator message to peer %d: %v", peer.id, err)
		}
	}

	// Start leader loop
	n.StartLeaderLoop()
}

func (n *Node) StartLeaderLoop() {
	log.Printf("Node %d starting leader loop\n", n.id)

}

func (n *Node) SetCurrentLeader(leaderId int) {
	n.peerLock.Lock()
	defer n.peerLock.Unlock()

	n.currentLeader = leaderId
}

func (n *Node) GetState() NodeState {
	n.peerLock.Lock()
	defer n.peerLock.Unlock()

	return n.state
}

func (n *Node) ChangeState(state NodeState) {
	n.peerLock.Lock()
	defer n.peerLock.Unlock()

	n.state = state
}

func (n *Node) CreateTopology(bullyNodes int) {
	for i := 1; i <= bullyNodes; i++ {
		if i == n.id {
			continue
		}
		peerIp := fmt.Sprintf("10.5.1.%d", i)
		peer := NewPeer(i, &peerIp)
		if peer != nil {
			n.peers = append(n.peers, peer)
		}
	}
}

// Listen will be executed by every node, it will keep an eye on incoming nodes that
// want to start a connection with this node. This communication will act as a client
// server communication, making this node the server and the incoming node the client.
// This allows us to break the full duplex communication, resulting in a more simple
// implementation.
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

// RespondToPeer will handle the communication with the incoming node, decoding the messages
// and responding them with the appropriate ones. This messages could be pings,
// elections or coordinations.
func (n *Node) RespondToPeer(conn *net.TCPConn) {
	log.Printf("Peer %s connected\n", conn.RemoteAddr().String())

	read := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)

	for {
		message := new(Message)
		err := read.Decode(message)

		if err != nil && err != io.EOF {
			fmt.Printf("Error decoding message: %v\n", err)
			if err := conn.Close(); err != nil {
				fmt.Printf("Error closing connection: %v\n", err)
			}
			return
		}

		if err == io.EOF {
			log.Printf("Peer %s disconnected\n", conn.RemoteAddr().String())
			if err := conn.Close(); err != nil {
				fmt.Printf("Error closing connection: %v\n", err)
			}
			return
		}

		n.handleMessage(message, encoder)
	}
}

func (n *Node) handleMessage(message *Message, encoder *gob.Encoder) {
	switch message.Type {
	case MessageTypePing:
		n.RespondToPing(message, encoder)
	case MessageTypeElection:
		n.RespondToElection(message, encoder)
		go n.StartElection()
	}
}

func (n *Node) RespondToPing(message *Message, encoder *gob.Encoder) {
	log.Printf("Received ping from %d, sending pong\n", message.PeerId)
	msg := Message{PeerId: n.id, Type: MessageTypePong, Seq: message.Seq}
	if err := encoder.Encode(msg); err != nil {
		fmt.Printf("Error sending pong message: %v\n", err)
	}
}
func (n *Node) RespondToElection(message *Message, encoder *gob.Encoder) {
	log.Printf("Received election from %d, sending ok\n", message.PeerId)
	msg := Message{PeerId: n.id, Type: MessageTypeOk, Seq: message.Seq}
	if err := encoder.Encode(msg); err != nil {
		fmt.Printf("Error sending ok message: %v\n", err)
	}
}

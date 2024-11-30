package main

import (
	"bullyresurrecter/shared"
	"context"
	"encoding/gob"
	"errors"
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

var PROCESS_LIST = [][]string{{"processes-1", "10.5.1.4"}, {"processes-2", "10.5.1.5"}, {"processes-3", "10.5.1.6"}, {"bully-node-1", "10.5.1.1"}, {"bully-node-2", "10.5.1.2"}, {"bully-node-3", "10.5.1.3"}}

type Node struct {
	name          string
	id            int
	peers         []*Peer
	serverConn    *net.TCPListener
	serverAddr    *net.TCPAddr
	lock          sync.Mutex
	state         NodeState
	currentLeader int
	stopContext   context.Context
	wg            sync.WaitGroup
	electionLock  *sync.Mutex
	stopLeader    chan struct{}
}

const INACTIVE_LEADER = -1

func NewNode(id int, stopContext context.Context) *Node {
	peers := []*Peer{}

	serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("10.5.1.%d:8000", id))
	if err != nil {
		fmt.Printf("Error resolving server address: %v\n", err)
		os.Exit(1)
	}

	return &Node{
		name:          fmt.Sprintf("bully-node-%d", id),
		id:            id,
		peers:         peers,
		serverAddr:    serverAddr,
		lock:          sync.Mutex{},
		state:         NodeStateFollower,
		currentLeader: INACTIVE_LEADER,
		stopContext:   stopContext,
		wg:            sync.WaitGroup{},
		electionLock:  &sync.Mutex{},
		stopLeader:    make(chan struct{}),
	}
}

func (n *Node) Close() {
	if n.serverConn != nil {
		if err := n.serverConn.Close(); err != nil {
			fmt.Printf("Error closing server connection: %v\n", err)
		}
	}
	// TODO: cerrar las conexiones con los peers
	for _, peer := range n.peers {
		peer.Close()
	}
}

func (n *Node) Run() {
	fmt.Printf("Running node %d\n", n.id)
	go shared.RunUDPListener(8080)
	n.wg.Add(1)
	go n.Listen() // solo responde por esta conexión TCP, no arranca la topología nunca (no PING, no ELECTION, etc)
	time.Sleep(2 * time.Second)
	go n.StartElection() // siempre queremos arrancar una eleccion cuando se levanta el nodo
	n.wg.Done()

	<-n.stopContext.Done()
}

func (n *Node) StartElection() {
	n.electionLock.Lock()
	defer n.electionLock.Unlock()
	n.ChangeState(NodeStateCandidate)

	// log.Printf("Node %d starting election\n", n.id)

	// Keep track of which peers we're waiting for responses from
	waitingResponses := make(map[int]bool)
	responseChan := make(chan int) // Channel to receive OK responses
	timeoutChan := time.After(shared.ElectionTimeout)

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
				if err := peer.conn.SetReadDeadline(time.Now().Add(shared.OkResponseTimeout)); err != nil {
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
		// log.Printf("No higher-numbered peers, becoming leader")
		go n.BecomeLeader()
		return
	}

	// Wait for responses or timeout and act accordingly
electionLoop:
	for len(waitingResponses) > 0 {
		select {
		case peerId := <-responseChan:
			delete(waitingResponses, peerId)
			// log.Printf("Received OK from peer %d", peerId)
			n.lock.Lock()
			if n.state == NodeStateCandidate {
				n.state = NodeStateWaitingCoordinator
				n.lock.Unlock()
				break electionLoop
			}
			n.lock.Unlock()

		case <-timeoutChan:
			// If we timeout waiting for responses, we become the leader if we are a candidate
			// or we wait for coordinator if we are waiting for one
			// log.Printf("Election timeout reached for node %d", n.id)
			if n.GetState() == NodeStateCandidate {
				// log.Printf("Node %d is a candidate and the election timed out, becoming leader", n.id)
				go n.BecomeLeader()
				return
			} else {
				// log.Printf("Node %d is not a candidate (state: %d), waiting for coordinator", n.id, n.GetState()) // recibio por lo menos un ok
				go n.waitForCoordinator()
				return
			}
		}
	}

	go n.waitForCoordinator()
}

func (n *Node) waitForCoordinator() {
	// log.Printf("Node %d waiting for coordinator\n", n.id)
	<-time.After(shared.ElectionTimeout + shared.OkResponseTimeout)
	if n.GetState() == NodeStateWaitingCoordinator {
		go n.StartElection()
	}
}

func (n *Node) startFollowerLoop(leaderId int) {
	log.Printf("Node %d following leader %d\n", n.id, leaderId)
	for {
		select {
		case <-n.stopContext.Done():
			return
		case <-time.After(shared.PingToLeaderTimeout):
			n.lock.Lock()
			if n.currentLeader != leaderId || n.state != NodeStateFollower {
				n.lock.Unlock()
				return
			}
			n.lock.Unlock()
			var leaderPeer *Peer
			for _, peer := range n.peers {
				if peer.id == leaderId {
					leaderPeer = peer
					break
				}
			}

			if leaderPeer == nil || leaderId == INACTIVE_LEADER || leaderPeer.conn == nil {
				// log.Printf("Leader peer not found, stopping follower loop and starting new election")
				go n.StartElection()
				return
			}
			// log.Printf("Node %d sending ping to leader %d\n", n.id, leaderId)
			if err := leaderPeer.Send(Message{PeerId: n.id, Type: MessageTypePing}); err != nil {
				log.Printf("Error sending ping to leader %d: %v", leaderId, err)
				continue
			}
			if err := leaderPeer.conn.SetReadDeadline(time.Now().Add(shared.PongTimeout)); err != nil {
				log.Printf("Error setting read deadline for leader %d: %v", leaderId, err)
				continue
			}
			response := new(Message)
			err := leaderPeer.decoder.Decode(response)
			if errors.Is(err, os.ErrDeadlineExceeded) || err == io.EOF {
				if n.GetCurrentLeader() != leaderId { // si es distinto, significa que el lider cambió y ya no hay que tirarle ping a ese
					continue
				}
				// log.Printf("Leader %d not responding, starting new election", leaderId)
				n.ChangeState(NodeStateFollower)
				n.SetCurrentLeader(INACTIVE_LEADER)
				go n.StartElection()
				return
			}

			if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
				log.Printf("Error decoding response from leader %d: %v", leaderId, err)
				continue
			}

			if response.Type == MessageTypePong {
				// log.Printf("Node %d received pong from leader %d\n", n.id, leaderId)
			}
		}
	}
}

func (n *Node) BecomeLeader() {
	oldLeader := n.GetCurrentLeader()
	n.ChangeState(NodeStateCoordinator)
	n.SetCurrentLeader(n.id)

	// Send coordinator message to all peers
	for _, peer := range n.peers {
		if err := peer.Send(Message{PeerId: n.id, Type: MessageTypeCoordinator}); err != nil {
			// log.Printf("Failed to send coordinator message to peer %d: %v", peer.id, err)
		}
	}

	// Start leader loop
	// log.Printf("Old leader: %d, new leader: %d\n", oldLeader, n.id)
	if oldLeader != n.id {
		go n.StartLeaderLoop()
	}
}

func (n *Node) StartLeaderLoop() {
	log.Printf("Node %d is the new leader\n", n.id)

	processList := make([][]string, 0)
	for _, process := range PROCESS_LIST {
		if process[0] == n.name {
			continue
		}
		processList = append(processList, process)
	}
	stopResurrecters, cancel := context.WithCancel(context.Background())
	resurrecter := NewResurrecter(processList, stopResurrecters)
	go resurrecter.Start()
	for {
		select {
		case <-n.stopContext.Done():
			cancel()
			return
		case <-n.stopLeader:
			// log.Printf("Stopping leader loop")
			resurrecter.Close()
			cancel()
			return
		}
	}
}

func (n *Node) GetCurrentLeader() int {
	// n.lock.Lock()
	// defer n.lock.Unlock()

	return n.currentLeader
}

func (n *Node) SetCurrentLeader(leaderId int) {
	// n.lock.Lock()
	// defer n.lock.Unlock()

	n.currentLeader = leaderId
}

func (n *Node) GetState() NodeState {
	// n.lock.Lock()
	// defer n.lock.Unlock()

	return n.state
}

func (n *Node) ChangeState(state NodeState) {
	// n.lock.Lock()
	// defer n.lock.Unlock()

	n.state = state
}

func (n *Node) CreateTopology(bullyNodes int) {
	for i := 1; i <= bullyNodes; i++ {
		if i == n.id {
			continue
		}
		peerName := fmt.Sprintf("bully-node-%d", i)
		peer := NewPeer(i, &peerName)
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
		select {
		case <-n.stopContext.Done():
			return
		default:
			conn, err := serverConn.AcceptTCP()
			if err != nil && errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				// fmt.Printf("Error accepting connection: %v\n", err)
				continue
			}
			go n.RespondToPeer(conn)
		}
	}
}

// RespondToPeer will handle the communication with the incoming node, decoding the messages
// and responding them with the appropriate ones. This messages could be pings,
// elections or coordinations.
func (n *Node) RespondToPeer(conn *net.TCPConn) {
	n.wg.Wait()
	// log.Printf("Peer %s connected\n", conn.RemoteAddr().String())

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
			break
		}

		if err == io.EOF {
			// log.Printf("Peer %s disconnected\n", conn.RemoteAddr().String())
			if err := conn.Close(); err != nil {
				fmt.Printf("Error closing connection: %v\n", err)
			}
			break
		}

		n.handleMessage(message, encoder)
	}
	log.Printf("Peer %s disconnected\n", conn.RemoteAddr().String())
}

// handleMessage implements the logic to respond to the different types of messages
// that the node receives from a peer. This set of messages only includes those that
// matter when acting as the server for incoming nodes (i.e. the messages a node sends
// to its peers, not the ones that come from the outside).
func (n *Node) handleMessage(message *Message, encoder *gob.Encoder) {
	switch message.Type {
	case MessageTypePing:
		n.handlePing(message, encoder)
	case MessageTypeElection:
		n.handleElection(message, encoder)
	case MessageTypeCoordinator:
		n.handleCoordinator(message)
	}
}

func (n *Node) handleCoordinator(message *Message) {
	n.lock.Lock()
	defer n.lock.Unlock()
	// log.Printf("Received coordinator from %d, state: %d", message.PeerId, n.state)

	if n.id > message.PeerId {
		// log.Printf("Node %d is higher than %d, starting new election", n.id, message.PeerId)
		go n.StartElection()
		return
	}

	if n.state == NodeStateCoordinator {
		log.Printf("Stopping leader loop")
		n.stopLeader <- struct{}{}
	}

	n.state = NodeStateFollower
	if n.currentLeader == message.PeerId {
		return
	}
	n.currentLeader = message.PeerId
	go n.startFollowerLoop(message.PeerId)
}

func (n *Node) handlePing(message *Message, encoder *gob.Encoder) {
	// log.Printf("Received ping from %d, sending pong\n", message.PeerId)
	msg := Message{PeerId: n.id, Type: MessageTypePong}
	if err := encoder.Encode(msg); err != nil {
		fmt.Printf("Error sending pong message: %v\n", err)
	}
}

func (n *Node) handleElection(message *Message, encoder *gob.Encoder) {
	// log.Printf("Received election from %d, sending ok\n", message.PeerId)
	msg := Message{PeerId: n.id, Type: MessageTypeOk}
	if err := encoder.Encode(msg); err != nil {
		fmt.Printf("Error sending ok message: %v\n", err)
	}
	go n.StartElection()
}

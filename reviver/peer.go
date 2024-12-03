package main // Representa a un PEER al que YO llamo y consulto, cuando el me consulta a mi usa su pripio call, no es full duplex
//
// A -> B y B -> A
// en vez de A <--> B
import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type Peer struct {
	id      int
	name    *string
	ip      *net.IP
	conn    *net.TCPConn
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func NewPeer(id int, name *string) *Peer {
	ipAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:8000", *name))
	if err != nil {
		return nil
	}

	return &Peer{id: id, name: name, ip: &ipAddr.IP, conn: nil, encoder: nil, decoder: nil}
}

func (p *Peer) call() error {
	connAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:8000", *p.name))
	if err != nil {
		return err
	}

	conn, err := net.DialTCP("tcp", nil, connAddr)
	if err != nil {
		return err
	}
	p.conn = conn
	p.encoder = gob.NewEncoder(conn)
	p.decoder = gob.NewDecoder(conn)
	return nil
}

func (p *Peer) Close() {
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}

func (p *Peer) Send(message Message) error {
	defer func() {
		recover() // a veces message es nil pero por alguna razon no se puede comparar con nil el parametro, haciendo que algunas veces paniquee ante mensajes perfectamente validos
	}()

	if p.conn == nil {
		if err := p.call(); err != nil {
			return err
		}
	}

	if err := p.encoder.Encode(message); err != nil {
		p.Close()
		return err
	}

	return nil
}

func (p *Peer) RecvTimeout(timeout time.Duration) (Message, error) {
	if p.conn == nil {
		return Message{}, errors.New(io.EOF.Error())
	}

	p.conn.SetReadDeadline(time.Now().Add(timeout))
	defer p.conn.SetReadDeadline(time.Time{})

	var message Message
	if err := p.decoder.Decode(&message); err != nil {
		return Message{}, err
	}
	return message, nil
}

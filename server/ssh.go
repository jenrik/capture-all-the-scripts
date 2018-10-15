package server

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"

	humanize "github.com/dustin/go-humanize"
	"golang.org/x/crypto/ssh"
)

type SSH struct {
	Path   string
	Events chan string

	banner []string

	lock             sync.RWMutex
	connections      map[net.Conn]*Connection
	totalconnections int
	bytessent        int
}

type State struct {
	Connections      []Connection
	TotalConnections int
	BytesSent        int
}

func (s *SSH) State() State {
	s.lock.RLock()
	defer s.lock.RUnlock()

	state := State{BytesSent: s.bytessent, TotalConnections: s.totalconnections}
	for _, conn := range s.connections {
		conn.BytesSent = conn.Written()
		state.Connections = append(state.Connections, *conn)
	}
	return state
}

func (s *SSH) Listen() {

	s.lock.Lock()
	s.connections = make(map[net.Conn]*Connection)
	s.lock.Unlock()

	err := s.preparebook("ebook.txt")
	if err != nil {
		panic(err)
	}

	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}

	hostkey, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},

		PublicKeyCallback: func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, fmt.Errorf("unknown public key for %q", c.User())
		},
		Banner: s.banner,
	}

	config.AddHostKey(hostkey)

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := net.Listen("tcp", s.Path)
	if err != nil {
		log.Fatal("failed to listen for connection: ", err)
	}

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Fatal("failed to accept incoming connection: ", err)
		}
		go s.acceptSSH(nConn, config)
	}

}

func (s *SSH) acceptSSH(nConn net.Conn, config *ssh.ServerConfig) {

	stateConn := Connection{Conn: nConn, Remote: nConn.RemoteAddr().String(), Started: time.Now()}
	s.lock.Lock()
	s.totalconnections++
	s.connections[nConn] = &stateConn
	s.lock.Unlock()

	_, _, _, err := ssh.NewServerConn(&stateConn, config)

	stateConn.Close()

	s.lock.Lock()
	delete(s.connections, nConn)
	s.bytessent += stateConn.Written()
	s.lock.Unlock()

	s.Events <- fmt.Sprintf("Disconnect: %s, (%s) took %s bytes: %s", stateConn.Remote, time.Now().Sub(stateConn.Started), humanize.Bytes(uint64(stateConn.Written())), err)
}

func (s *SSH) preparebook(path string) error {
	fd, err := os.Open(path)
	if err != nil {
		return err
	}

	defer fd.Close()

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		// append line and add back newline
		s.banner = append(s.banner, fmt.Sprintf("%s\n", scanner.Text()))
	}
	return scanner.Err()
}
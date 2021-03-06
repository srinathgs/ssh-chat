package sshd

import (
	"net"

	"github.com/shazow/rateio"
	"golang.org/x/crypto/ssh"
)

// Container for the connection and ssh-related configuration
type SSHListener struct {
	net.Listener
	config *ssh.ServerConfig

	RateLimit   func() rateio.Limiter
	HandlerFunc func(term *Terminal)
}

// Make an SSH listener socket
func ListenSSH(laddr string, config *ssh.ServerConfig) (*SSHListener, error) {
	socket, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, err
	}
	l := SSHListener{Listener: socket, config: config}
	return &l, nil
}

func (l *SSHListener) handleConn(conn net.Conn) (*Terminal, error) {
	if l.RateLimit != nil {
		// TODO: Configurable Limiter?
		conn = ReadLimitConn(conn, l.RateLimit())
	}

	// Upgrade TCP connection to SSH connection
	sshConn, channels, requests, err := ssh.NewServerConn(conn, l.config)
	if err != nil {
		return nil, err
	}

	// FIXME: Disconnect if too many faulty requests? (Avoid DoS.)
	go ssh.DiscardRequests(requests)
	return NewSession(sshConn, channels)
}

// Accept incoming connections as terminal requests and yield them
func (l *SSHListener) Serve() {
	defer l.Close()
	for {
		conn, err := l.Accept()

		if err != nil {
			logger.Printf("Failed to accept connection: %v", err)
			break
		}

		// Goroutineify to resume accepting sockets early
		go func() {
			term, err := l.handleConn(conn)
			if err != nil {
				logger.Printf("Failed to handshake: %v", err)
				return
			}
			l.HandlerFunc(term)
		}()
	}
}

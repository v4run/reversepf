package remote

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/v4run/reversepf/internal/commands"
)

type ControlServer struct {
	controlMessageConn     net.Conn
	controlMessageConnLock *sync.RWMutex
	messagesToLocal        chan []byte
	messagesFromLocal      chan []byte
	logger                 *log.Logger
	Port                   string
}

func (s *ControlServer) Start() {
	listener, err := net.Listen("tcp", net.JoinHostPort("", s.Port))
	if err != nil {
		s.logger.Fatal("Error starting listener", "err", err)
	}
	s.logger.Info("Ready to accept connection", "addr", listener.Addr().String())
	for {
		conn, err := listener.Accept()
		if err != nil {
			s.logger.Error("Error accepting connection", "err", err)
			continue
		}
		s.logger.Info("Received new connection request", "addr", conn.RemoteAddr().String())
		s.controlMessageConnLock.RLock()
		if s.controlMessageConn == nil {
			s.controlMessageConnLock.RUnlock()
			s.controlMessageConnLock.Lock()
			if s.controlMessageConn == nil {
				s.controlMessageConn = conn
				go s.handleControlMessages()
			}
			s.controlMessageConnLock.Unlock()
			continue
		}
		s.controlMessageConnLock.RUnlock()
		fmt.Fprintf(conn, "Client connection already established. Only one client can be connected at a time")
		conn.Close()
	}
}

func (s *ControlServer) handleControlMessages() {
	s.logger.Info("Control message handler started")
	defer s.logger.Info("Control message handler terminated")
	reader := bufio.NewReader(s.controlMessageConn)
	killer := make(chan struct{})
	go func() {
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				s.controlMessageConnLock.Lock()
				s.controlMessageConn.Close()
				s.controlMessageConn = nil
				s.controlMessageConnLock.Unlock()
				close(killer)
			}
			s.messagesFromLocal <- line
		}
	}()
	for {
		select {
		case <-killer:
			return
		case msg := <-s.messagesToLocal:
			s.controlMessageConnLock.RLock()
			s.controlMessageConn.Write(msg)
			s.controlMessageConnLock.RUnlock()
		}
	}
}

func (s *ControlServer) SendMessage(command commands.Command) error {
	s.controlMessageConnLock.RLock()
	defer s.controlMessageConnLock.RUnlock()
	if s.controlMessageConn == nil {
		return errors.New("client not connected yet")
	}
	s.messagesToLocal <- command.Bytes()
	return nil
}

func NewControlServer(port string) ControlServer {
	return ControlServer{
		controlMessageConn:     nil,
		controlMessageConnLock: new(sync.RWMutex),
		messagesToLocal:        make(chan []byte),
		messagesFromLocal:      make(chan []byte),
		logger:                 log.WithPrefix("[CTRLSRV]"),
		Port:                   port,
	}
}

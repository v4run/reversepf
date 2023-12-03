package remote

import (
	"fmt"
	"io"
	"net"

	"github.com/charmbracelet/log"
	"github.com/v4run/reversepf/internal/commands"
)

type Service struct {
	getLocalConn   func() net.Conn
	sendControlMsg func(commands.Command) error
	logger         *log.Logger
	Port           string
}

func (s *Service) Start() {
	listner, err := net.Listen("tcp", net.JoinHostPort("", s.Port))
	if err != nil {
		s.logger.Fatal("Error starting listner", "err", err)
	}
	s.logger.Info("Ready to accept connections", "addr", listner.Addr().String())
	for {
		conn, err := listner.Accept()
		if err != nil {
			s.logger.Error("Error accepting connection", err)
			continue
		}
		s.logger.Info("Received new connection request", "addr", conn.RemoteAddr().String())
		if err := s.sendControlMsg(commands.NewInitCommand()); err != nil {
			s.logger.Error("Error sending control message", "err", err)
			fmt.Fprintf(conn, "Local component not ready. Please retry.")
			conn.Close()
			continue
		}
		go s.proxyData(conn, s.getLocalConn())
	}
}

func (s *Service) proxyData(conn, portalConn net.Conn) {
	serviceAddr := conn.RemoteAddr().String()
	portalAddr := portalConn.RemoteAddr().String()
	s.logger.Info("New proxy established", "serviceAddr", serviceAddr, "portalAddr", portalAddr)
	defer conn.Close()
	go func() {
		defer portalConn.Close()
		if _, err := io.Copy(portalConn, conn); err != nil {
			s.logger.Warn("Connection closed", "err", err)
		}
	}()
	if _, err := io.Copy(conn, portalConn); err != nil {
		s.logger.Warn("Connection closed", "err", err)
	}
	s.logger.Info("Stopping proxy", "serviceAddr", serviceAddr, "portalAddr", portalAddr)
}

func NewService(port string, getLocalConn func() net.Conn, sendControlMsg func(commands.Command) error) Service {
	if sendControlMsg == nil {
		log.Fatal("Error create new service. `sendControlMsg` is nil")
	}
	if getLocalConn == nil {
		log.Fatal("Error create new service. `localConnChan` is nil")
	}
	return Service{
		getLocalConn:   getLocalConn,
		sendControlMsg: sendControlMsg,
		logger:         log.WithPrefix("[SERVICE]"),
		Port:           port,
	}
}

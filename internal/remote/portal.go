package remote

import (
	"net"

	"github.com/charmbracelet/log"
)

type Portal struct {
	logger   *log.Logger
	connChan chan net.Conn
	Port     string
}

func (p *Portal) Start() {
	listener, err := net.Listen("tcp", net.JoinHostPort("", p.Port))
	if err != nil {
		p.logger.Fatal("Error starting listener", "err", err)
	}
	p.logger.Info("Ready to accept connection", "addr", listener.Addr().String())
	for {
		conn, err := listener.Accept()
		if err != nil {
			p.logger.Error("Error accepting connection", "err", err)
			continue
		}
		p.logger.Info("Received new connection request", "addr", conn.RemoteAddr().String())
		p.connChan <- conn
	}
}

func (p *Portal) Connection() net.Conn {
	return <-p.connChan
}

func NewPortal(port string) Portal {
	return Portal{
		connChan: make(chan net.Conn),
		Port:     port,
		logger:   log.WithPrefix("[PRTLSRV]"),
	}
}

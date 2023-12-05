package local

import (
	"bufio"
	"errors"
	"io"
	"net"
	"time"

	"github.com/charmbracelet/log"
	"github.com/v4run/reversepf/internal/commands"
)

type Local struct {
	controlServerPort string
	portalPort        string
	localServicePort  string
}

func NewLocalComponent(localServicePort, portalPort, controlServerPort string) Local {
	return Local{
		controlServerPort: controlServerPort,
		portalPort:        portalPort,
		localServicePort:  localServicePort,
	}
}

func (l Local) Start() {
	l.establishControlServerConnection()
}

func (l Local) establishControlServerConnection() {
	var (
		err  error
		conn net.Conn
	)
	log.Info("Establishing control server connection", "controlServerPort", l.controlServerPort)
	for {
		for {
			conn, err = net.Dial("tcp", net.JoinHostPort("", l.controlServerPort))
			if err != nil {
				log.Warn("Waiting for control server to start")
				time.Sleep(time.Second * 3)
				continue
			}
			log.Info("Established connection to control server")
			break
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			command, err := commands.ReadCommand(reader)
			if err != nil {
				log.Error("Error getting/processing command from remote", "err", err)
				if errors.Is(err, io.EOF) {
					break
				}
				continue
			}
			log.Info("New command received from remote", "command", command)
			switch command.Type {
			case commands.TypeInit:
				go l.handleInitCommand()
			default:
			}
		}
		log.Info("Client disconnected")
	}
}

func (l Local) handleInitCommand() {
	log.Info("Starting a new proxy connection", "portalPort", l.portalPort, "localPort", l.localServicePort)
	portalConn, err := net.Dial("tcp", net.JoinHostPort("", l.portalPort))
	if err != nil {
		log.Error("Unable to connect to portal", "err", err)
		return
	}
	defer portalConn.Close()
	localConn, err := net.Dial("tcp", net.JoinHostPort("", l.localServicePort))
	if err != nil {
		log.Error("Unable to connect to local", "err", err)
		return
	}
	defer localConn.Close()
	go func() {
		defer localConn.Close()
		if _, err := io.Copy(localConn, portalConn); err != nil {
			log.Warn("Error proxying", "err", err)
			return
		}
	}()
	if _, err := io.Copy(portalConn, localConn); err != nil {
		log.Warn("Error proxying", "err", err)
		return
	}
	log.Info("Proxy connection terminated")
}

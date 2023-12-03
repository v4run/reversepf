package utils

import (
	"net"
	"os"
	"os/signal"
)

func GetRandomOpenPort(count int) ([]string, error) {
	var (
		ports     []string
		listeners []net.Listener
	)
	for i := 0; i < count; i++ {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			return nil, err
		}
		_, port, err := net.SplitHostPort(l.Addr().String())
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	for _, l := range listeners {
		l.Close()
	}
	return ports, nil
}

func HandleSignals(cb func(), sigs ...os.Signal) {
	sigChan := make(chan os.Signal, len(sigs))
	signal.Notify(sigChan, sigs...)
	go func() {
		<-sigChan
		cb()
	}()
}

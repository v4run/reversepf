/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/v4run/reversepf/internal/commands"
	"github.com/v4run/reversepf/utils"
)

var (
	localPort   string
	remotePort  string
	kubeContext string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "reversepf",
	Short: "Makes a local port accessible from inside a remote server",
	Long: `This tool makes a local port accessible from inside a remote server.
It has two parts. A local and a remote part. Both the parts works together to make the port accessible.

Local
This part proxies traffic between the local service port and the "portal-port".

Remote
This part runs in the remote server and proxies traffic there.`,
	Run: func(_ *cobra.Command, _ []string) {
		if remotePort == "" {
			remotePort = localPort
		}
		var err error
		if controlServerPort == "" {
			controlServerPort, err = utils.GetRandomOpenPort()
			if err != nil {
				log.Fatal("Error getting open port for control server", "err", err)
			}
		}
		log.Info(
			"Initializing service",
			"localPort", localPort,
			"remotePort", remotePort,
			"portalPort", portalPort,
			"controlServerPort", controlServerPort,
		)
		var conn net.Conn
		for {
			for {
				conn, err = net.Dial("tcp", net.JoinHostPort("", controlServerPort))
				if err != nil {
					log.Error("Unable to connect to control server. retrying...", "err", err)
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
					go handleInitCommand()
				default:
				}
			}
			log.Info("client disconnected")
		}
	},
}

func handleInitCommand() {
	log.Info("Starting a new proxy connection", "portalPort", portalPort, "localPort", localPort)
	portalConn, err := net.Dial("tcp", net.JoinHostPort("", portalPort))
	if err != nil {
		log.Error("Unable to connect to portal", "err", err)
		return
	}
	defer portalConn.Close()
	localConn, err := net.Dial("tcp", net.JoinHostPort("", localPort))
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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&localPort, "local-port", "l", "", "Local port to be forwarded")
	rootCmd.Flags().StringVarP(&remotePort, "remote-port", "r", "", "Port the kubernetes service should expose. If not specified, it will be same as local-port.")
	rootCmd.Flags().StringVarP(&portalPort, "portal-port", "p", "", "The portal-port in remote server")
	rootCmd.Flags().StringVarP(&kubeContext, "kube-context", "", "", "Name of the kubeconfig context to use")
	rootCmd.Flags().StringVarP(&controlServerPort, "control-server-port", "c", "", "The port on which control server listens")
	rootCmd.MarkFlagRequired("local-port")
}

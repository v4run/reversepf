package cmd

import (
	"github.com/spf13/cobra"
	"github.com/v4run/reversepf/internal/remote"
)

var (
	servicePort       string
	controlServerPort string
	portalPort        string
)

// remoteCmd represents the remote command
var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "The remote component that is run on the remote server",
	Long: `This component is run on the remote server. It has three sub components:
	- Service
	- Portal
	- Control Server

Service
Other services in the remote server should connect to this component. It listens on "service-port".

Portal
The "portal" listens for new connections and proxies all the traffic to the service. It listens on "portal-port".
So, the "portal-port" should be accessible from the local machine.

Control Server
"control server" is used to transfer control messages. It listen on "control-server-port". So, "control-server-port" 
also should be accessible from local machine.
	`,
	Run: func(_ *cobra.Command, _ []string) {
		portal := remote.NewPortal(portalPort)
		controlServer := remote.NewControlServer(controlServerPort)
		service := remote.NewService(servicePort, portal.Connection, controlServer.SendMessage)
		go portal.Start()
		go controlServer.Start()
		service.Start()
	},
}

func init() {
	rootCmd.AddCommand(remoteCmd)
	remoteCmd.Flags().StringVarP(&servicePort, "service-port", "s", "", "The port on which the service is exposed")
	remoteCmd.Flags().StringVarP(&controlServerPort, "control-server-port", "c", "", "The port on which control server listens")
	remoteCmd.Flags().StringVarP(&portalPort, "portal-port", "p", "", "The port to which the local client component connects")
	remoteCmd.MarkFlagRequired("service-port")
	remoteCmd.MarkFlagRequired("control-server-port")
	remoteCmd.MarkFlagRequired("local-client-port")
}

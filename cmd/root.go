package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/v4run/reversepf/version"
)

var (
	localPort string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Version: version.Version,
	Use:     "reversepf",
	Short:   "Makes a local port accessible from inside a remote server",
	Long: `This tool makes a local port accessible from inside a remote server.
It has two parts. A local and a remote part. Both the parts works together to make the port accessible.

Local
This part proxies traffic between the local service port and the "portal-port".

Remote
This part runs in the remote server and proxies traffic there.`,
	Example: "reversepf --name demo k8s -l 8888",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

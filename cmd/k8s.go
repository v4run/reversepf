package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/v4run/reversepf/internal/k8s"
	"github.com/v4run/reversepf/internal/local"
	"github.com/v4run/reversepf/utils"
	"github.com/v4run/reversepf/version"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/homedir"
)

const (
	AppName = "reversepf"
)

var (
	kubeContext string
	kubeconfig  string
	name        string
)

// k8sCmd represents the k8s command
var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "The local part for k8s remote",
	Long:  `The part creates a new deployment, service and pod in the remote k8s. Then the control-server-port and portal-port ports are port forwarded to local.`,
	Run: func(_ *cobra.Command, _ []string) {
		ctx := context.Background()
		if servicePort == "" {
			servicePort = localPort
		}
		ports, err := utils.GetRandomOpenPort(2)
		if err != nil {
			log.Error("Error getting random open ports", "err", err)
			return
		}
		if controlServerPort == "" {
			controlServerPort = ports[0]
		}
		if portalPort == "" {
			portalPort = ports[1]
		}
		if name == "" {
			name = rand.String(8)
		}
		namespace := fmt.Sprintf("%s-%s", AppName, name)
		k8sConfig := k8s.Config{
			AppName:           AppName,
			Namespace:         namespace,
			Version:           version.Version,
			ControlServerPort: controlServerPort,
			PortalPort:        portalPort,
			ServicePort:       servicePort,
			Kubeconfig:        kubeconfig,
			KubeContext:       kubeContext,
		}
		deployer := k8s.NewDeployer(k8sConfig)
		utils.HandleSignals(func() {
			deployer.Cleanup(ctx)
			os.Exit(0)
		}, syscall.SIGINT)
		if err := deployer.Deploy(ctx); err != nil {
			log.Error("Error setting up remote components", "err", err)
			return
		}
		localComponent := local.NewLocalComponent(localPort, portalPort, controlServerPort)
		localComponent.Start()
	},
}

func init() {
	rootCmd.AddCommand(k8sCmd)
	k8sCmd.Flags().StringVarP(&localPort, "local-port", "l", "", "Local port to be forwarded")
	k8sCmd.Flags().StringVarP(&portalPort, "portal-port", "p", "", "The portal-port in remote server")
	k8sCmd.Flags().StringVarP(&controlServerPort, "control-server-port", "c", "", "The port on which control server listens")
	k8sCmd.Flags().StringVarP(&kubeContext, "context", "", "", "The name of the kubeconfig context to use")
	k8sCmd.Flags().StringVarP(&servicePort, "service-port", "s", "", "The port on which the service is exposed. If not specified, local-port is used")
	k8sCmd.Flags().StringVarP(&name, "name", "n", "", "The name of this specific run. Reuse a name to replace older instance. If no name is specified a random string is used instead")
	if home := homedir.HomeDir(); home == "" {
		k8sCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "", "", "Path to the kubeconfig file to use for requests")
		k8sCmd.MarkFlagRequired("context")
	} else {
		k8sCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "", filepath.Join(home, ".kube", "config"), "Path to the kubeconfig file to use for requests")
	}
	k8sCmd.MarkFlagRequired("local-port")
}

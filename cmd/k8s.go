package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/v4run/reversepf/internal/k8s"
	"github.com/v4run/reversepf/utils"
	"github.com/v4run/reversepf/version"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/client-go/util/homedir"
	"k8s.io/utils/ptr"
)

const (
	K8sPrefix = "reversepf"
)

var (
	kubeContext string
	kubeconfig  string
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
		k8sConfig := k8s.Config{
			Key:               "",
			Version:           version.Version,
			ControlServerPort: controlServerPort,
			PortalPort:        portalPort,
			ServicePort:       servicePort,
		}
		deployer := newDeployer()
		utils.HandleSignals(func() {
			deployer.cleanup(ctx, k8sConfig)
			os.Exit(0)
		}, syscall.SIGINT)
		if err := deployer.DeployRemoteComponents(ctx, k8sConfig); err != nil {
			return
		}
		if err := deployer.forwardPorts(ctx, k8sConfig, controlServerPort, portalPort); err != nil {
			log.Error("Error forwarding ports", "err", err)
			return
		}
		establishControlServerConnection()
	},
}

type Deployer struct {
	client  *dynamic.DynamicClient
	decoder runtime.Serializer
	mapper  *restmapper.DeferredDiscoveryRESTMapper
	config  *rest.Config
}

func (d Deployer) cleanup(ctx context.Context, config k8s.Config) {
	log.Info("Cleaning up remote resources")
	namespaceRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	if err := d.client.Resource(namespaceRes).Delete(ctx, K8sPrefix+config.Key, metav1.DeleteOptions{}); err != nil {
		log.Error("Unable to do cleanup. Please do the cleanup manually", "err", err)
	}
}

func (d Deployer) deploy(
	ctx context.Context,
	manifest string,
) error {
	var obj unstructured.Unstructured
	_, gvk, err := d.decoder.Decode([]byte(manifest), nil, &obj)
	if err != nil {
		return err
	}
	mapping, err := d.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}
	var dr dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		// namespaced resources should specify the namespace
		dr = d.client.Resource(mapping.Resource).Namespace(obj.GetNamespace())
	} else {
		// for cluster-wide resources
		dr = d.client.Resource(mapping.Resource)
	}
	data, err := json.Marshal(obj.Object)
	if err != nil {
		return err
	}
	_, err = dr.Patch(
		ctx,
		obj.GetName(),
		types.ApplyPatchType,
		data,
		metav1.PatchOptions{
			FieldManager: K8sPrefix + "-k8s",
			Force:        ptr.To(true),
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func newDeployer() Deployer {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext},
	).ClientConfig()
	if err != nil {
		log.Fatal("Error building k8s config", "err", err)
	}
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		log.Fatal("Error creating discovery client", "err", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Fatal("Error building k8s config", "err", err)
	}
	var decoder = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	deployer := Deployer{
		client:  client,
		decoder: decoder,
		mapper:  mapper,
		config:  cfg,
	}
	return deployer
}

func (d Deployer) DeployRemoteComponents(ctx context.Context, k8sConfig k8s.Config) error {
	log.Info("Deploying remote resources")
	log.Info("Deploying new namespace", "namespace", K8sPrefix+k8sConfig.Key)
	if err := d.deploy(ctx, executeTemplate(k8s.Namespace, k8sConfig)); err != nil {
		log.Error("Error deploying remote components", "err", err)
		return err
	}
	log.Info("Deploying new deployment", "namespace", K8sPrefix+k8sConfig.Key)
	if err := d.deploy(ctx, executeTemplate(k8s.Deployment, k8sConfig)); err != nil {
		log.Error("Error deploying remote components", "err", err)
		return err
	}
	log.Info("Deploying new service", "namespace", K8sPrefix+k8sConfig.Key)
	if err := d.deploy(ctx, executeTemplate(k8s.Service, k8sConfig)); err != nil {
		log.Error("Error deploying remote components", "err", err)
		return err
	}
	return nil
}

func (d Deployer) getPodName(ctx context.Context, config k8s.Config) string {
	log.Info("Getting pod details")
	for {
		namespaceRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
		list, err := d.client.Resource(namespaceRes).Namespace(K8sPrefix+config.Key).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Fatal("Error getting pod list", "err", err)
		}
		for _, u := range list.Items {
			podStatus, _, err := unstructured.NestedFieldCopy(u.Object, "status", "phase")
			if err != nil {
				log.Fatal("Error getting pod details", "err", err)
			}
			if podStatus == "Running" {
				name := u.GetName()
				log.Info("Pod is Running", "name", name)
				return name
			}
		}
		time.Sleep(time.Second * 2)
		log.Info("Pod not ready yet. Retrying")
	}
}

func (d Deployer) forwardPorts(ctx context.Context, config k8s.Config, ports ...string) error {
	transport, upgrader, err := spdy.RoundTripperFor(d.config)
	if err != nil {
		return err
	}
	url, err := url.Parse(d.config.Host)
	if err != nil {
		return err
	}
	go func() {
		for {
			podName := d.getPodName(ctx, config)
			url.Path = path.Join("api", "v1", "namespaces", K8sPrefix+config.Key, "pods", podName, "portforward")
			dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)
			var formattedPorts []string
			for _, p := range ports {
				formattedPorts = append(formattedPorts, fmt.Sprintf("%s:%s", p, p))
			}
			stopChan := make(chan struct{})
			readyChan := make(chan struct{})
			forwarder, err := portforward.New(dialer, formattedPorts, stopChan, readyChan, io.Discard, os.Stderr)
			if err != nil {
				log.Error("Error creating new forwarder. Retrying", "err", err)
			} else {
				if err := forwarder.ForwardPorts(); err != nil {
					log.Error("Error forwarding ports. Retrying", "err", err)
				}
			}
			time.Sleep(time.Second * 5)
		}
	}()
	return nil
}

func executeTemplate(templateName string, config k8s.Config) string {
	var buf bytes.Buffer
	if err := k8s.Template.ExecuteTemplate(&buf, templateName, config); err != nil {
		log.Fatal("Error executing template", "template", templateName, "err", err)
	}
	return buf.String()
}

func init() {
	rootCmd.AddCommand(k8sCmd)
	k8sCmd.Flags().StringVarP(&localPort, "local-port", "l", "", "Local port to be forwarded")
	k8sCmd.Flags().StringVarP(&portalPort, "portal-port", "p", "", "The portal-port in remote server")
	k8sCmd.Flags().StringVarP(&controlServerPort, "control-server-port", "c", "", "The port on which control server listens")
	k8sCmd.Flags().StringVarP(&kubeContext, "context", "", "", "The name of the kubeconfig context to use")
	k8sCmd.Flags().StringVarP(&servicePort, "service-port", "s", "", "The port on which the service is exposed. If not specified, local-port is used")
	if home := homedir.HomeDir(); home == "" {
		k8sCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "", "", "Path to the kubeconfig file to use for requests")
		k8sCmd.MarkFlagRequired("context")
	} else {
		k8sCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "", filepath.Join(home, ".kube", "config"), "Path to the kubeconfig file to use for requests")
	}
	k8sCmd.MarkFlagRequired("local-port")
}
package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
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
	"k8s.io/utils/ptr"
)

type Deployer struct {
	client    *dynamic.DynamicClient
	decoder   runtime.Serializer
	mapper    *restmapper.DeferredDiscoveryRESTMapper
	config    *rest.Config
	k8sConfig Config
}

func (d Deployer) Cleanup(ctx context.Context) {
	log.Info("Cleaning up remote resources")
	namespaceRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	if err := d.client.Resource(namespaceRes).Delete(ctx, d.k8sConfig.Namespace, metav1.DeleteOptions{}); err != nil {
		log.Error("Unable to do cleanup. Please do the cleanup manually", "err", err)
	}
}

func (d Deployer) Deploy(ctx context.Context) error {
	if err := d.DeployRemoteComponents(ctx); err != nil {
		return err
	}
	if readChanChan, err := d.ForwardPorts(ctx, d.k8sConfig.ControlServerPort, d.k8sConfig.PortalPort); err != nil {
		log.Error("Error forwarding ports", "err", err)
		return err
	} else {
		go func() {
			for r := range readChanChan {
				<-r
				printConnectionDetails(fmt.Sprintf("%s.%s:%s", d.k8sConfig.AppName, d.k8sConfig.Namespace, d.k8sConfig.ServicePort))
			}
		}()
	}
	return nil
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
			FieldManager: d.k8sConfig.AppName + "-k8s",
			Force:        ptr.To(true),
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func NewDeployer(k8sConfig Config) Deployer {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: k8sConfig.Kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: k8sConfig.KubeContext},
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
		client:    client,
		decoder:   decoder,
		mapper:    mapper,
		config:    cfg,
		k8sConfig: k8sConfig,
	}
	return deployer
}

func (d Deployer) DeployRemoteComponents(ctx context.Context) error {
	log.Info("Deploying remote resources")
	var (
		err  error
		tmpl string
	)
	log.Info("Deploying new namespace", "name", d.k8sConfig.Namespace)
	tmpl, err = executeTemplate(Namespace, d.k8sConfig)
	if err != nil {
		return err
	}
	if err = d.deploy(ctx, tmpl); err != nil {
		log.Error("Error deploying remote components", "err", err)
		return err
	}
	log.Info("Deploying new deployment", "namespace", d.k8sConfig.Namespace)
	tmpl, err = executeTemplate(Deployment, d.k8sConfig)
	if err != nil {
		return err
	}
	if err = d.deploy(ctx, tmpl); err != nil {
		log.Error("Error deploying remote components", "err", err)
		return err
	}
	log.Info("Deploying new service", "namespace", d.k8sConfig.Namespace)
	tmpl, err = executeTemplate(Service, d.k8sConfig)
	if err != nil {
		return err
	}
	if err = d.deploy(ctx, tmpl); err != nil {
		log.Error("Error deploying remote components", "err", err)
		return err
	}
	return nil
}

func (d Deployer) getPodName(ctx context.Context, k8sConfig Config) string {
	log.Info("Getting pod details")
	for {
		namespaceRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
		list, err := d.client.Resource(namespaceRes).Namespace(k8sConfig.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Warn("Error getting pod list", "err", err)
		}
		for _, u := range list.Items {
			podStatus, _, err := unstructured.NestedFieldCopy(u.Object, "status", "phase")
			if err != nil {
				log.Warn("Error getting pod details", "err", err)
			}
			if podStatus == "Running" {
				podName := u.GetName()
				log.Info("Pod is Running", "name", podName)
				return podName
			}
		}
		time.Sleep(time.Second * 2)
		log.Info("Pod not ready yet")
	}
}

func (d Deployer) ForwardPorts(ctx context.Context, ports ...string) (chan chan struct{}, error) {
	transport, upgrader, err := spdy.RoundTripperFor(d.config)
	if err != nil {
		return nil, err
	}
	url, err := url.Parse(d.config.Host)
	if err != nil {
		return nil, err
	}
	readChanChan := make(chan chan struct{})
	go func() {
		for {
			podName := d.getPodName(ctx, d.k8sConfig)
			url.Path = path.Join("api", "v1", "namespaces", d.k8sConfig.Namespace, "pods", podName, "portforward")
			dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)
			var formattedPorts []string
			for _, p := range ports {
				formattedPorts = append(formattedPorts, fmt.Sprintf("%s:%s", p, p))
			}
			readyChan := make(chan struct{})
			readChanChan <- readyChan
			forwarder, err := portforward.New(dialer, formattedPorts, nil, readyChan, io.Discard, os.Stderr)
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
	return readChanChan, nil
}

var connectionDetailsStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	Foreground(lipgloss.AdaptiveColor{Light: "236", Dark: "253"}).
	PaddingTop(1).
	PaddingBottom(1).
	PaddingLeft(2).
	PaddingRight(2)

func printConnectionDetails(details string) {
	fmt.Println(connectionDetailsStyle.Render(details))
}

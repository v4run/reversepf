package k8s

import (
	"bytes"
	"text/template"

	"github.com/charmbracelet/log"
)

var tmplt = template.New("k8s-manifests")

func init() {
	if _, err := tmplt.New(Namespace).Parse(namespace); err != nil {
		log.Fatal("Error parsing template", "err", err, "template", "Namespace")
	}
	if _, err := tmplt.New(Service).Parse(service); err != nil {
		log.Fatal("Error parsing template", "err", err, "template", "Service")
	}
	if _, err := tmplt.New(Deployment).Parse(deployment); err != nil {
		log.Fatal("Error parsing template", "err", err, "template", "Deployment")
	}
}

type Config struct {
	AppName           string
	Namespace         string
	Version           string
	ControlServerPort string
	PortalPort        string
	ServicePort       string
	Kubeconfig        string
	KubeContext       string
}

const (
	Namespace  = "Namespace"
	Service    = "Service"
	Deployment = "Deployment"
)

const namespace = `
apiVersion: v1
kind: Namespace
metadata:
  name: {{.Namespace}}
`

const deployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.AppName}}
  namespace: {{.Namespace}}
  labels:
    app: {{.AppName}}
spec:
  selector:
    matchLabels:
      app: {{.AppName}}
  replicas: 1
  template:
    metadata:
      labels:
        app: {{.AppName}}
    spec:
      containers:
        - name: {{.AppName}}
          image: v4run/{{.AppName}}:{{.Version}}
          imagePullPolicy: IfNotPresent
          args:
            - "remote"
            - "-c"
            - "{{.ControlServerPort}}"
            - "-p"
            - "{{.PortalPort}}"
            - "-s"
            - "{{.ServicePort}}"
          resources:
            requests:
              cpu: 100m
              memory: 100Mi
      restartPolicy: Always
`

const service = `
apiVersion: v1
kind: Service
metadata:
  name: {{.AppName}}
  namespace: {{.Namespace}}
spec:
  selector:
    app: {{.AppName}}
  ports:
    - port: {{.ControlServerPort}}
      name: control-server
      protocol: TCP
    - port: {{.PortalPort}}
      name: portal-port
      protocol: TCP
    - port: {{.ServicePort}}
      name: service
      protocol: TCP
`

func executeTemplate(templateName string, config Config) (string, error) {
	var buf bytes.Buffer
	if err := tmplt.ExecuteTemplate(&buf, templateName, config); err != nil {
		log.Error("Error executing template", "template", templateName, "err", err)
		return "", err
	}
	return buf.String(), nil
}

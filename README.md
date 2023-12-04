# reversepf

Makes a local port available in your remote server. Currently supports Kubernetes only.

## Building

```bash
go build
```

## Running

```bash
reversepf --name demo k8s -l 8888 
# makes the local port 3306 available in the default k8s cluster at reversepf.reversepf-demo:8888
```

## Demo

![Demo](./assets/demo.gif)

# reversepf

Makes a local port available in you remote server. Currently supports kubernets only.

## Building

```bash
go build
```

## Running

```bash
reversepf k8s -l 8888
# makes the local port 3306 available in the default k8s cluster at reversepf.reversepf:8888
```

## Demo

![Demo](./assets/demo.gif)

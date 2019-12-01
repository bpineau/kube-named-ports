# kube-named-ports

[![Build Status](https://github.com/bpineau/kube-named-ports/workflows/CI/badge.svg)](https://github.com/bpineau/kube-named-ports/actions)
[![Coverage Status](https://coveralls.io/repos/github/bpineau/kube-named-ports/badge.svg?branch=master)](https://coveralls.io/github/bpineau/kube-named-ports?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/bpineau/kube-named-ports)](https://goreportcard.com/report/github.com/bpineau/kube-named-ports)

Declare GCP named ports on GKE node pools's instance groups according to Kubernetes services annotations.

## Annotations

The services annotation is format is as such:

```yaml
annotations:
  kube-named-ports.io/port-name: "newport6666"
  kube-named-ports.io/port-value: "6666"
```

Or you can add several ports in one annotation with an inline json dictionnary:
```yaml
annotations:
  kube-named-ports.io/port-map: '{ "foo": 1234, "bar": 5678, "baz": 9876 }'
```

## Build

Assuming you have go 1.13.4 (or up) :

```shell
make build
```

### Usage

`cluster` is mandatory, the remaining can be automatically guessed when running
in cluster, from hosts instance's metadata and serviceaccount.

```
Usage:
  kube-named-ports [flags]
  kube-named-ports [command]

Available Commands:
  help        Help about any command
  version     Print the version number

Flags:
  -s, --api-server string      kube api server url
  -n, --cluster string         cluster name (mandatory)
  -c, --config string          configuration file (default "/etc/knp/kube-named-ports.yaml")
  -d, --dry-run                dry-run mode
  -p, --healthcheck-port int   port for answering healthchecks
  -h, --help                   help for kube-named-ports
  -k, --kube-config string     kube config path
  -v, --log-level string       log level (default "debug")
  -o, --log-output string      log output (default "stderr")
  -r, --log-server string      log server (if using syslog)
  -j, --project string         project (optional when in cluster, can be found in host's metadata
  -i, --resync-interval int    resync interval in seconds (default 900)
  -z, --zone string            cluster zone name (optional, can be guessed)
```

## Docker image

A ready to use, public docker image is available at [Docker Hub](https://hub.docker.com/r/bpineau/kube-named-ports/), published at each release.
You can use it directly from your Kubernetes deployments, ie.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kube-named-ports
  namespace: kube-system
  labels:
    k8s-app: kube-named-ports
spec:
  selector:
    matchLabels:
      k8s-app: kube-named-ports
  replicas: 1
  template:
    metadata:
      labels:
        k8s-app: kube-named-ports
    spec:
      containers:
        - name: kube-named-ports
          image: bpineau/kube-named-ports:0.5.0
          args:
            - --cluster=MySuperCluster
            - --healthcheck-port=8080
          resources:
            requests:
              cpu: 0.1
              memory: 50Mi
            limits:
              cpu: 0.2
              memory: 100Mi
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            timeoutSeconds: 5
            initialDelaySeconds: 10
```


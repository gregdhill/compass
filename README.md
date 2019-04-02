# Compass

[![Go Report Card](https://goreportcard.com/badge/github.com/gregdhill/compass)](https://goreportcard.com/report/github.com/monax/compass)

A cloud native pipeline and templating tool. Simply describe how the environment should be setup, and it will find a direction for your stack. As it is still in early development, please use with caution.

## Features

- Combine Kubernetes Specifications & Helm Charts
  - Install, upgrade & delete cloud resources.
  - Build a pipeline with dependencies and requirements.
  - Combine with shell scripts.
- Layer Go Templates
  - Inject key:value pairs through the command-line.
  - Render intermediate input templates.
  - Render final resource input.
  - Handy Go Functions

## Installation

You'll need Go (version >= 1.11) [installed and correctly setup](https://golang.org/doc/install) first.

```bash
go get github.com/monax/compass
compass --help
```

## Getting Started

We'll need a [YAML](https://yaml.org) configuration file I like to call a scroll...

```yaml
# scroll.yaml
values:
  namespace: default
  image: ipfs/go-ipfs
  tag: v0.4.9
  add: true

stages:
  ipfs:
    kind: helm
    release: my-release
    repository: stable
    name: ipfs
    input: values.yaml

  add:
    kind: kube
    depends:
    - ipfs
    requires:
    - add
    input: manifest.yaml
```

If you save that as `scroll.yaml` you'll see that two other files named `values.yaml` and `spec.yaml` are required, so let's go ahead and create them:

```yaml
# values.yaml
{{ $ipfs_auth := (printf "https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull" .image) }}
image: {{ printf "%s@sha256" .image }}:{{ getDigest "https://index.docker.io" .image .tag (getAuth $ipfs_auth) }}

replicaCount: 2
persistence:
  enabled: true
  size: "8Gi"
```

```yaml
# manifest.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .ipfs_release }}-add
spec:
  template:
    spec:
      containers:
      - name: add-object
        image: appropriate/curl
        imagePullPolicy: Always
        command: ["/bin/sh", "-c", 'curl -F file=@entrypoint.sh "http://{{ .ipfs_release }}:5001/api/v0/add"']
      restartPolicy: OnFailure
  backoffLimit: 1
```

This workflow will bootstrap a two node IPFS setup on your cluster and run a job to populate it with a file. This works because the [stable chart](https://github.com/helm/charts/tree/master/stable/ipfs/) actually sets up a service for the running deployment which can be reached using the name of the release. Naturally extending the definitions supported by Helm, this job could also have been suited as a `post-install` hook but it's easier to just declare it here as a dependency. With custom templating we can share definitions across applications and add in overlay functions such as `getDigest` which ensures that we always get the latest SHA hash for the given docker tag. If, on creation, we decided not to run the job, we can just remove the `add` value.

```bash
compass scroll.yaml
```

## Advanced

There are many more pipeline options:

```yaml
# scroll.yaml
stages:
  one:
    # helm stuff
    kind: helm
    release: my-release-1
    namespace: default
    repository: stable
    name: chart_one
    # once installed, don't upgrade
    abandon: true
    # read this input template
    input: values1.yaml

  two:
    kind: helm
    release: my-release-2
    namespace: default
    repository: stable
    name: chart_two
    # requirements not met, don't install
    requires:
    - some_key
    input: values2.yaml

  three:
    kind: kubernetes
    namespace: default
    # bash scripts to run before and after
    jobs:
      before:
      - this.sh
      after:
      - that.sh
    # add extra values only for this stage
    values:
      key: value
    input: manifest.yaml

  four:
    kind: kube
    namespace: default
    input: manifest.yaml
    # wait for three to install / upgrade
    depends:
    - three
    # then delete this object
    remove: true
```

And a number of helpful templating functions:

```
getDigest <server> <repo> <tag> <auth_token>
getAuth <url>
fromConfigMap <name> <namespace> <key>
fromSecret <name> <namespace> <key>
parseJSON <input> <keys...>
readEnv <envname>
readFile <filename>
```
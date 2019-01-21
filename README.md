# Compass

[![Go Report Card](https://goreportcard.com/badge/github.com/gregdhill/compass)](https://goreportcard.com/report/github.com/gregdhill/compass)

Inspired by [bashful](https://github.com/wagoodman/bashful), compass is a declarative pipelining and templating tool for Helm. Simply describe how the environment should be setup, and it will chart out a direction for your stack.

## Installation

```bash
go get github.com/gregdhill/compass
```

## Getting Started

We'll need a [YAML](https://yaml.org) configuration file I like to call a scroll...

```yaml
values:
  imageRepo: "docker/image"
  imageTag: "latest"

charts:
  - release: my-release
    namespace: default
    repo: stable
    name: chart
    template: values.yaml
```

If you save that as `scroll.yaml` you'll see that another file named `values.yaml` is required, so let's go ahead and create that:

```yaml
image:
  repository: {{ .imageRepo }}
  tag: {{ .imageTag }}
  pullPolicy: Always
```

This is designed to mimic the `values.yaml` required by most Helm charts, but it also allows us to add an extra layer of templating on top. If any stage of your workflow does not require any custom values, you're free to leave this blank. Nevertheless, any additional arguments specified in the global values of our `scroll.yaml` can also be overridden with the `-env` flag of the following command. Let's build what we have so far:

```bash
compass scroll.yaml
```

This will setup `stable/chart` in namespace `default` with the name `my-release`, i.e.:

```bash
helm upgrade --install my-release stable/chart --namespace=default --set 'repository="docker/image",tag="latest",pullPolicy=Always'
```

Though the footprint is arguably minimal for most deployments / upgrades, the aim is to simply multi-chart, multi-environment workflows which can't be supported by simple bash multi-liners.

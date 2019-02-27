# Compass

[![Go Report Card](https://goreportcard.com/badge/github.com/gregdhill/compass)](https://goreportcard.com/report/github.com/monax/compass)

Inspired by [bashful](https://github.com/wagoodman/bashful), compass is a declarative pipelining and templating tool for Helm. Simply describe how the environment should be setup, and it will chart out a direction for your stack(s). As it is still in early development, please use with caution.

## Features

- [x] Stack Creation / Destruction
- [x] Chart Dependencies & Variable Requirements
- [x] Install & Forget Chart
- [x] Fetch Docker Digest By Tag
- [x] Pre/Post-Deployment Bash Jobs
- [x] Explicit or Global Values (Namespace, Release, Version)
- [x] Derive Values From Extra Init Template
- [x] Output JSON Values

## Installation

```bash
go get github.com/monax/compass
```

## Getting Started

We'll need a [YAML](https://yaml.org) configuration file I like to call a scroll...

```yaml
# scroll.yaml
values:
  imageRepo: "docker/image"
  imageTag: "latest"

charts:
  test:
  - release: my-release
    namespace: default
    repo: stable
    name: chart
    template: values.yaml
```

If you save that as `scroll.yaml` you'll see that another file named `values.yaml` is required, so let's go ahead and create that:

```yaml
# values.yaml
image:
  repository: {{ .imageRepo }}
  tag: {{ .imageTag }}
  pullPolicy: Always
```

This is designed to mimic the `values.yaml` required by most Helm charts, but it also allows us to add an extra layer of templating on top. Additional arguments can also be added from a file specified by the `--import` flag. Let's build what we have so far:

```bash
compass scroll.yaml
```

This will setup `stable/chart` in namespace `default` with the name `my-release`, analogous to:

```bash
helm upgrade --install my-release stable/chart --namespace=default --set 'repository="docker/image",tag="latest",pullPolicy=Always'
```

However the aim here is to simplify multi-chart, multi-environment workflows (i.e. production vs staging).

## Advanced

Let's dive into a deeper example:

```yaml
# scroll.yaml
values:
  imageRepo: "docker/image"
  imageTag: "latest"
  environment: "production"

charts:
  test1:
  - release: my-release-1
    namespace: default
    repo: stable
    name: chart_one
    template: values1.yaml
  test2:
  - release: my-release-2
    namespace: default
    repo: stable
    name: chart_two
    template: values2.yaml
    depends:
    - test1
    jobs:
      after:
      - ./script/publish.sh
```

```yaml
# values1.yaml
image:
  repository: {{ .imageRepo }}@sha256
{{ if eq .environment "production" }}
  tag: {{ digest .docker_url .imageRepo "master" "DOCKER_TOKEN" }}
{{ else }}
  tag: {{ digest .docker_url .imageRepo "develop" "DOCKER_TOKEN" }}
{{ end }}
```

Executing `compass scroll.yaml` will first prepare two releases with one dependency (`test1 -> test2`). When rendering the first values template it will traverse the `production` logic which calls a function named `digest` on the `master` tag. This fetches the latest digest for that release tag from the targeted docker API to ensure that Kubernetes collects our most up-to-date image. Once that has finished installing it will trigger the `test2` deployment. This has a post deployment job which calls a simple bash script called `publish.sh`. This will also have access to all values used in the pipeline such as `.imageRepo`.

To get a quick glimpse of what values are generated from your pipeline, use the following command:

```bash
compass scroll.yaml -export
```
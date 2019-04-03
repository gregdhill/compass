package core

import (
	"sync"
	"testing"

	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/stretchr/testify/assert"
)

func newTestChart() *Stage {
	return &Stage{
		Forget: false,
		Kind:   "helm",
		Resource: &helm.Chart{
			Name:       "burrow",
			Repository: "stable",
			Version:    "",
			Namespace:  "test-namespace",
			Release:    "test-release",
			Bridge:     helm.NewFakeBridge(),
		},
		K8s: kube.NewFakeK8s(),
	}
}

var testJob = `
apiVersion: batch/v1
kind: Job
metadata:
  name: test-job
spec:
  template:
    spec:
      containers:
      - name: test
        image: alpine:latest
        imagePullPolicy: Always
        command: ["/bin/sh", "-c", "exit 0"]
      restartPolicy: OnFailure
  backoffLimit: 1
`

func newTestManifest() *Stage {
	k8s := kube.NewFakeK8s()
	stg := Stage{
		Forget: false,
		Kind:   "kube",
		Resource: &kube.Manifest{
			Namespace: "test-namespace",
			Object:    []byte(testJob),
			K8s:       k8s,
		},
	}
	return &stg
}

func TestShellJobs(t *testing.T) {
	jobs := []string{"echo hello"}
	shellJobs(nil, jobs, true)

	jobs = []string{"error 1"}
	assert.Panics(t, func() { shellJobs(nil, jobs, false) })
}

func TestCreateDestroyChart(t *testing.T) {
	chart := newTestChart()

	wgs := make(Depends, 1)
	var w sync.WaitGroup
	w.Add(1)
	wgs["test"] = &w

	values := make(map[string]string, 1)
	err := chart.Forward("test", values, &wgs, false, false)
	assert.NoError(t, err)

	err = chart.Backward("test", values, &wgs, false, false)
	assert.NoError(t, err)
}

func TestCreateDestroyManifest(t *testing.T) {
	man := newTestManifest()

	wgs := make(Depends, 1)
	var w sync.WaitGroup
	w.Add(1)
	wgs["test"] = &w

	values := make(map[string]string, 1)
	err := man.Forward("test", values, &wgs, false, false)
	assert.NoError(t, err)

	err = man.Backward("test", values, &wgs, false, false)
	assert.NoError(t, err)
}

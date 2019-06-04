package core

import (
	"sync"
	"testing"

	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func newTestChart() *Stage {
	stg := &Stage{
		Forget: false,
		Kind:   "helm",
		Resource: &helm.Chart{
			Name:      "stable/burrow",
			Version:   "",
			Namespace: "test-namespace",
			Release:   "test-release",
		},
	}
	stg.Connect(helm.NewFakeClient())
	return stg
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
	k8s := kube.NewFakeClient()
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
	shellJobs(nil, jobs)

	jobs = []string{"error 1"}
	assert.Panics(t, func() { shellJobs(nil, jobs) })
}

func TestCreateDestroyChart(t *testing.T) {
	chart := newTestChart()

	wgs := make(Depends, 1)
	var w sync.WaitGroup
	w.Add(1)
	wgs["test"] = &w

	logger := logrus.New().WithField("kind", chart.Kind)
	values := make(util.Values, 1)
	err := chart.Forward(logger, "test", values, &wgs, false)
	assert.NoError(t, err)

	err = chart.Backward(logger, "test", values, &wgs, false)
	assert.NoError(t, err)
}

func TestCreateDestroyManifest(t *testing.T) {
	man := newTestManifest()

	wgs := make(Depends, 1)
	var w sync.WaitGroup
	w.Add(1)
	wgs["test"] = &w

	logger := logrus.New().WithField("kind", man.Kind)
	values := make(util.Values, 1)
	err := man.Forward(logger, "test", values, &wgs, false)
	assert.NoError(t, err)

	err = man.Backward(logger, "test", values, &wgs, false)
	assert.NoError(t, err)
}

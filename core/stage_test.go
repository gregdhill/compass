package core

import (
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
			Namespace: "test-namespace",
			Release:   "test-release",
		},
	}
	stg.Connect(helm.NewFakeClient())
	return stg
}

var testConf = `
apiVersion: v1
kind: ConfigMap
data:
  test: "data"
metadata:
  creationTimestamp: null
  name: config-data
type: Opaque
`

func newTestManifest() *Stage {
	k8s := kube.NewFakeClient()
	stg := Stage{
		Forget: false,
		Kind:   "kube",
		Resource: &kube.Manifest{
			Namespace: "test-namespace",
			Object:    []byte(testConf),
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

	logger := logrus.New().WithField("kind", chart.Kind)
	values := make(util.Values, 1)
	err := chart.Create(logger, "test", values, false)
	assert.NoError(t, err)

	err = chart.Destroy(logger, "test", values, false)
	assert.NoError(t, err)
}

func TestCreateDestroyManifest(t *testing.T) {
	man := newTestManifest()

	logger := logrus.New().WithField("kind", man.Kind)
	values := make(util.Values, 1)
	err := man.Create(logger, "test", values, false)
	assert.NoError(t, err)

	err = man.Destroy(logger, "test", values, false)
	assert.NoError(t, err)
}

package core

import (
	"testing"

	"github.com/monax/compass/core/schema"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func newTestChart() *schema.Stage {
	stg := &schema.Stage{
		Actions: schema.Actions{
			Forget: false,
			Kind:   "helm",
		},
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

func newTestManifest() *schema.Stage {
	k8s := kube.NewFakeClient()
	stg := schema.Stage{
		Actions: schema.Actions{
			Forget: false,
			Kind:   "kube",
		},
		Resource: &kube.Manifest{
			Namespace: "test-namespace",
			Object:    []byte(testConf),
			K8s:       k8s,
		},
	}
	return &stg
}

func TestShellTasks(t *testing.T) {
	jobs := []string{"echo hello"}
	err := shellTasks(jobs, nil)
	assert.NoError(t, err)

	jobs = []string{"error 1"}
	err = shellTasks(jobs, nil)
	assert.Error(t, err)
}

func TestCreateDestroyChart(t *testing.T) {
	chart := newTestChart()

	logger := logrus.New().WithField("kind", chart.Kind)
	values := make(util.Values, 1)
	err := Create(chart, logger, "test", values, false)
	assert.NoError(t, err)

	err = Destroy(chart, logger, "test", values, false)
	assert.NoError(t, err)
}

func TestCreateDestroyManifest(t *testing.T) {
	man := newTestManifest()

	logger := logrus.New().WithField("kind", man.Kind)
	values := make(util.Values, 1)
	err := Create(man, logger, "test", values, false)
	assert.NoError(t, err)

	err = Destroy(man, logger, "test", values, false)
	assert.NoError(t, err)
}

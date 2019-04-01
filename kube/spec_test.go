package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestManifest() Manifest {
	return Manifest{
		Namespace: "test-namespace",
		K8s:       NewFakeK8s(),
	}
}

var testData = `
apiVersion: v1
kind: Secret
data:
  test: "data"
metadata:
  creationTimestamp: null
  name: secret-data
type: Opaque
`

func TestCreateDelete(t *testing.T) {
	m := newTestManifest()

	err := m.K8s.CreateNamespace(m.Namespace)
	assert.NoError(t, err)

	m.SetInput([]byte(testData))
	err = m.Install()
	assert.NoError(t, err)

	exists, err := m.Status()
	assert.NoError(t, err)
	assert.Equal(t, true, exists)

	err = m.Delete()
	assert.NoError(t, err)

	exists, err = m.Status()
	assert.Error(t, err)
	assert.Equal(t, false, exists)
}

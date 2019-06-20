package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestManifest() Manifest {
	return Manifest{
		Namespace: "test-namespace",
		K8s:       NewFakeClient(),
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

---

apiVersion: v1
kind: ConfigMap
data:
  test: "data"
metadata:
  creationTimestamp: null
  name: config-data
type: Opaque
`

func TestCreateDelete(t *testing.T) {
	m := newTestManifest()

	err := m.K8s.CreateNamespace(m.Namespace)
	assert.NoError(t, err)

	m.SetInput([]byte(testData))
	err = m.InstallOrUpgrade()
	assert.NoError(t, err)

	exists, err := m.Status()
	assert.NoError(t, err)
	assert.Equal(t, true, exists)

	err = m.Delete()
	assert.NoError(t, err)

	exists, err = m.Status()
	assert.NoError(t, err)
	assert.Equal(t, false, exists)
}

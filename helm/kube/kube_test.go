package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestK8s() *K8s {
	k := K8s{}
	k.client = fake.NewSimpleClientset()
	return &k
}

func TestFindTiller(t *testing.T) {
	k8s := newTestK8s()

	assert.Panics(t, func() { k8s.FindPod("tiller", "kube-system") })

	n := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}
	_, err := k8s.client.Core().Namespaces().Create(n)
	if err != nil {
		t.Errorf("Error injecting namespace into fake client: %v", err)
	}

	p := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "tiller-test", Labels: map[string]string{"name": "tiller"}}}
	_, err = k8s.client.Core().Pods("kube-system").Create(p)
	if err != nil {
		t.Errorf("Error injecting pod into fake client: %v", err)
	}

	assert.Equal(t, "tiller-test", k8s.FindPod("tiller", "kube-system"))
}

package kube

import (
	"bytes"
	"fmt"

	"github.com/monax/compass/util"
	v1batch "k8s.io/api/batch/v1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"

	// import all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Manifest represents a kubernetes definition
type Manifest struct {
	Namespace string `yaml:"namespace"` // namespace
	Timeout   int64  `yaml:"timeout"`   // install / upgrade wait time
	Remove    bool   `yaml:"remove"`    // remove once installed
	Object    []byte
	*K8s
}

// Lint checks that our definition has a namespace
func (m *Manifest) Lint(key string, in *util.Values) error {
	if m.Namespace = in.Cascade(key, "namespace", m.Namespace); m.Namespace == "" {
		return fmt.Errorf("namespace for %s is empty", key)
	}
	return nil
}

// SetInput adds to object to the manifest
func (m *Manifest) SetInput(obj []byte) {
	m.Object = obj
}

// GetInput return the manifest object
func (m *Manifest) GetInput() []byte {
	return m.Object
}

// Connect links the manifest to the k8 api
func (m *Manifest) Connect(k8s interface{}) {
	m.K8s = k8s.(*K8s)
}

func (m *Manifest) buildObjects() ([]runtime.Object, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	objs := bytes.Split(m.Object, []byte("---"))
	var specs []runtime.Object
	for _, obj := range objs {
		spec, _, err := decode(obj, nil, nil)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

type action string

const (
	install action = "install"
	upgrade action = "upgrade"
	status  action = "status"
	delete  action = "delete"
)

// Execute performs actions against the kubernetes api
func (m *Manifest) Execute(spec runtime.Object, do action) error {
	gvk := spec.GetObjectKind().GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version}

	// this is empty if using the fake client
	groupResources, err := restmapper.GetAPIGroupResources(m.K8s.typed.Discovery())
	if err != nil {
		return err
	}

	if len(groupResources) != 0 {
		gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
		rm := restmapper.NewDiscoveryRESTMapper(groupResources)
		mapping, err := rm.RESTMapping(gk, gvk.Version)
		if err != nil {
			return err
		}
		gvr = mapping.Resource
	}

	resourceInterface := m.K8s.dynamic.Resource(gvr).Namespace(m.Namespace)

	// convert the object to unstructured
	unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(spec)
	if err != nil {
		return err
	}
	obj := unstructured.Unstructured{Object: unstruct}

	switch action(do) {
	case install:
		_, err = resourceInterface.Create(&obj, metav1.CreateOptions{})
	case upgrade:
		_, err = resourceInterface.Update(&obj, metav1.UpdateOptions{})
	case status:
		_, err = resourceInterface.Get(obj.GetName(), metav1.GetOptions{})
	case delete:
		err = resourceInterface.Delete(obj.GetName(), &metav1.DeleteOptions{})
	default:
		return fmt.Errorf("action type '%v' unknown", do)
	}

	if err == nil {
		switch def := spec.(type) {
		case *v1batch.Job:
			err = m.waitJob(m.Namespace, def.GetName(), m.Remove, m.Timeout)
		case *v1core.Pod:
			err = m.waitPod(m.Namespace, def.GetName(), m.Remove, m.Timeout)
		}
	}

	return err
}

// Workflow executes against each kubernetes spec
func (m *Manifest) Workflow(do action) error {
	specs, err := m.buildObjects()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		if spec != nil {
			if err = m.Execute(spec, do); err != nil {
				return err
			}
		}
	}
	return nil
}

// Install the decoded kubernetes objects
func (m *Manifest) Install() error {
	if m.Namespace == "" {
		ns, _, _ := m.base.Namespace()
		m.Namespace = ns
	}
	return m.Workflow(install)
}

// Upgrade the decoded kubernetes objects
func (m *Manifest) Upgrade() error {
	return m.Workflow(upgrade)
}

// Status returns true if the objects exists
func (m *Manifest) Status() (bool, error) {
	err := m.Workflow(status)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Delete the decoded kubernetes objects
func (m *Manifest) Delete() error {
	return m.Workflow(delete)
}

package kube

import (
	"fmt"

	"github.com/monax/compass/util"
	v1batch "k8s.io/api/batch/v1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	// import all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Manifest represents a Kubernetes definition
type Manifest struct {
	Namespace string `yaml:"namespace"` // namespace
	Timeout   int64  `yaml:"timeout"`   // install / upgrade wait time
	Wait      bool   `yaml:"wait"`
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

// Connect links the manifest to the k8 api
func (m *Manifest) Connect(k8s interface{}) {
	m.K8s = k8s.(*K8s)
}

func (m *Manifest) objToSpec() (runtime.Object, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	spec, _, err := decode(m.Object, nil, nil)
	return spec, err
}

type Action string

const (
	Install Action = "install"
	Upgrade Action = "upgrade"
	Status  Action = "status"
	Delete  Action = "delete"
)

func (m *Manifest) Execute(do Action) error {
	var err error
	var spec runtime.Object

	spec, err = m.objToSpec()
	if err != nil {
		return err
	}

	switch def := spec.(type) {

	case *v1core.ConfigMap:
		switch Action(do) {
		case Install:
			_, err = m.client.Core().ConfigMaps(m.Namespace).Create(def)

		case Upgrade:
			_, err = m.client.Core().ConfigMaps(m.Namespace).Update(def)

		case Status:
			_, err = m.client.Core().ConfigMaps(m.Namespace).Get(def.GetName(), metav1.GetOptions{})

		case Delete:
			err = m.client.Core().ConfigMaps(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{})

		default:
			return fmt.Errorf("action type '%v' unknown", do)
		}

	case *v1batch.Job:
		switch Action(do) {
		case Install:
			if _, err = m.client.Batch().Jobs(m.Namespace).Create(def); err != nil {
				break
			}

			if m.Wait {
				if err = m.waitJob(def.GetName(), m.Namespace, m.Timeout); err != nil {
					break
				}
			}

		case Upgrade:
			_, err = m.client.Batch().Jobs(m.Namespace).Update(def)

		case Status:
			_, err = m.client.Batch().Jobs(m.Namespace).Get(def.GetName(), metav1.GetOptions{})

		case Delete:
			var policy metav1.DeletionPropagation = "Foreground"
			err = m.client.Batch().Jobs(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{PropagationPolicy: &policy})

		default:
			return fmt.Errorf("action type '%v' unknown", do)
		}

	case *v1core.PersistentVolumeClaim:
		switch Action(do) {
		case Install:
			_, err = m.client.Core().PersistentVolumeClaims(m.Namespace).Create(def)

		case Upgrade:
			// can't upgrade pvc
			break

		case Status:
			_, err = m.client.Core().PersistentVolumeClaims(m.Namespace).Get(def.GetName(), metav1.GetOptions{})

		case Delete:
			err = m.client.Core().PersistentVolumeClaims(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{})

		default:
			return fmt.Errorf("action type '%v' unknown", do)
		}

	case *v1core.Pod:
		switch Action(do) {
		case Install:
			if _, err = m.client.Core().Pods(m.Namespace).Create(def); err != nil {
				break
			}

			if m.Wait {
				if err = m.waitPod(def.GetName(), m.Namespace, m.Timeout); err != nil {
					break
				}
			}

		case Upgrade:
			_, err = m.client.Core().Pods(m.Namespace).Update(def)

		case Status:
			_, err = m.client.Core().Pods(m.Namespace).Get(def.GetName(), metav1.GetOptions{})

		case Delete:
			err = m.client.Core().Pods(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{})

		default:
			return fmt.Errorf("action type '%v' unknown", do)
		}

	case *v1core.Secret:
		switch Action(do) {
		case Install:
			_, err = m.client.Core().Secrets(m.Namespace).Create(def)

		case Upgrade:
			_, err = m.client.Core().Secrets(m.Namespace).Update(def)

		case Status:
			_, err = m.client.Core().Secrets(m.Namespace).Get(def.GetName(), metav1.GetOptions{})

		case Delete:
			err = m.client.Core().Secrets(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{})

		default:
			return fmt.Errorf("action type '%v' unknown", do)
		}

	case *v1core.Service:
		switch Action(do) {
		case Install:
			_, err = m.client.Core().Services(m.Namespace).Create(def)

		case Upgrade:
			_, err = m.client.Core().Services(m.Namespace).Update(def)

		case Status:
			_, err = m.client.Core().Services(m.Namespace).Get(def.GetName(), metav1.GetOptions{})

		case Delete:
			err = m.client.Core().Services(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{})

		default:
			return fmt.Errorf("action type '%v' unknown", do)
		}

	default:
		return fmt.Errorf("object type '%v' unknown", def)
	}

	return err
}

// Install the decoded Kubernetes object
func (m *Manifest) Install() error {
	return m.Execute(Install)
}

// Upgrade the decoded Kubernetes object
func (m *Manifest) Upgrade() error {
	return m.Execute(Upgrade)
}

// Status returns true if the object exists
func (m *Manifest) Status() (bool, error) {
	err := m.Execute(Status)
	if err != nil {
		return false, err
	}
	return true, err
}

// Delete the decoded Kubernetes object
func (m *Manifest) Delete() error {
	return m.Execute(Delete)
}

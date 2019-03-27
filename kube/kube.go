package kube

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/monax/compass/util"
	"github.com/valyala/fastjson"
	v1batch "k8s.io/api/batch/v1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	// import all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// K8s represents a connection to kubernetes
type K8s struct {
	client kubernetes.Interface
	config *rest.Config
}

// NewK8s populates a new connection
func NewK8s() *K8s {
	// Fetch in-cluster config, if err try local
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			panic(err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return &K8s{client, config}
}

// NewFakeK8s returns a testing instance
func NewFakeK8s() *K8s {
	return &K8s{client: fake.NewSimpleClientset()}
}

// FindPod finds a pod based on the namespace and the name label
func (k8s *K8s) FindPod(name, namespace string) (result string, err error) {
	pods, err := k8s.client.Core().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("name=%s", name)})
	if len(pods.Items) < 1 {
		return result, errors.New("tiller not found")
	}
	return pods.Items[0].Name, err
}

// ForwardPod establishes a persistent connection to a remote pod
func (k8s *K8s) ForwardPod(name, namespace, port string) chan struct{} {
	roundTripper, upgrader, err := spdy.RoundTripperFor(k8s.config)
	if err != nil {
		panic(err)
	}

	tillerName, err := k8s.FindPod(name, namespace)
	if err != nil {
		panic(err)
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", "kube-system", tillerName)
	hostIP := strings.TrimLeft(k8s.config.Host, "https://")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	// ports = local, remote
	forwarder, err := portforward.New(dialer, []string{port}, stopChan, readyChan, out, errOut)
	if err != nil {
		panic(err)
	}

	go func() {
		for range readyChan {
		}
		if len(errOut.String()) != 0 {
			panic(errOut.String())
		}
	}()
	go forwarder.ForwardPorts()

	return stopChan
}

// FromConfigMap reads an entry from a ConfigMap
func (k8s *K8s) FromConfigMap(name, namespace, key string) (result string, err error) {
	cm, err := k8s.client.Core().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if cm == nil {
		return result, errors.New("failed to get configmap")
	}
	return cm.Data[key], nil
}

// FromSecret reads an entry from a Secret
func (k8s *K8s) FromSecret(name, namespace, key string) (result string, err error) {
	sec, err := k8s.client.Core().Secrets(namespace).Get(name, metav1.GetOptions{})
	if sec == nil {
		return result, errors.New("failed to get secret")
	}
	return string(sec.Data[key]), nil
}

// CreateNamespace tells the k8s api to make a namespace
func (k8s *K8s) CreateNamespace(name string) error {
	ns := &v1core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := k8s.client.Core().Namespaces().Create(ns)
	return err
}

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

func (m *Manifest) waitJob(jobName string) error {
	// cleanup job and associated pods
	var policy metav1.DeletionPropagation = "Foreground"
	defer m.client.Batch().Jobs(m.Namespace).Delete(jobName, &metav1.DeleteOptions{PropagationPolicy: &policy})

	// make a watcher to check if this job succeeds or fails
	watch, err := m.client.Batch().Jobs(m.Namespace).Watch(metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", jobName), TimeoutSeconds: &m.Timeout})
	if err != nil {
		return err
	}

	for event := range watch.ResultChan() {
		if event.Object.(*v1batch.Job).Status.Succeeded != 0 {
			return nil
		} else if event.Object.(*v1batch.Job).Status.Failed != 0 {

			// due to failure as we delete the pods, get all logs
			pods, err := m.client.Core().Pods(m.Namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", jobName)})
			req := m.client.Core().Pods(m.Namespace).GetLogs(pods.Items[0].GetName(), &v1core.PodLogOptions{})
			readCloser, err := req.Stream()
			if err != nil {
				return err
			}
			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, readCloser)
			if err != nil {
				return err
			}

			return fmt.Errorf("failed to deploy %s\n%s", jobName, buf.String())
		}
	}
	return nil
}

func (m *Manifest) objToSpec() (runtime.Object, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	spec, _, err := decode(m.Object, nil, nil)
	return spec, err
}

// Install the decoded Kubernetes object
func (m *Manifest) Install() error {
	spec, err := m.objToSpec()
	if err != nil {
		return err
	}

	switch def := spec.(type) {
	case *v1batch.Job:
		if _, err := m.client.Batch().Jobs(m.Namespace).Create(def); err != nil {
			return err
		}

		if m.Wait {
			jobName := def.GetName()
			if err := m.waitJob(jobName); err != nil {
				return err
			}
		}

		return nil

	case *v1core.PersistentVolumeClaim:
		_, err := m.client.Core().PersistentVolumeClaims(m.Namespace).Create(def)
		return err

	case *v1core.ConfigMap:
		_, err := m.client.Core().ConfigMaps(m.Namespace).Create(def)
		return err

	case *v1core.Secret:
		_, err := m.client.Core().Secrets(m.Namespace).Create(def)
		return err

	default:
		return fmt.Errorf("object type '%v' unknown", def)
	}
}

// Upgrade the decoded Kubernetes object
func (m *Manifest) Upgrade() error {
	spec, err := m.objToSpec()
	if err != nil {
		return err
	}

	switch def := spec.(type) {
	case *v1batch.Job:
		_, err := m.client.Batch().Jobs(m.Namespace).Update(def)
		return err

	case *v1core.PersistentVolumeClaim:
		// can't upgrade pvc
		return nil

	case *v1core.ConfigMap:
		_, err := m.client.Core().ConfigMaps(m.Namespace).Update(def)
		return err

	case *v1core.Secret:
		_, err := m.client.Core().Secrets(m.Namespace).Update(def)
		return err

	default:
		return fmt.Errorf("object type '%v' unknown", def)
	}
}

// Status returns true if the object exists
func (m *Manifest) Status() (bool, error) {
	spec, err := m.objToSpec()
	if err != nil {
		return false, err
	}

	switch def := spec.(type) {
	case *v1batch.Job:
		_, err := m.client.Batch().Jobs(m.Namespace).Get(def.GetName(), metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		return false, nil

	case *v1core.PersistentVolumeClaim:
		_, err := m.client.Core().PersistentVolumeClaims(m.Namespace).Get(def.GetName(), metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		return false, nil

	case *v1core.ConfigMap:
		_, err := m.client.Core().ConfigMaps(m.Namespace).Get(def.GetName(), metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		return false, nil

	case *v1core.Secret:
		_, err := m.client.Core().Secrets(m.Namespace).Get(def.GetName(), metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		return false, nil

	default:
		return false, fmt.Errorf("object type '%v' unknown", def)
	}
}

// Delete the decoded Kubernetes object
func (m *Manifest) Delete() error {
	spec, err := m.objToSpec()
	if err != nil {
		return err
	}

	switch def := spec.(type) {
	case *v1batch.Job:
		var policy metav1.DeletionPropagation = "Foreground"
		return m.client.Batch().Jobs(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{PropagationPolicy: &policy})

	case *v1core.PersistentVolumeClaim:
		return m.client.Core().PersistentVolumeClaims(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{})

	case *v1core.ConfigMap:
		return m.client.Core().ConfigMaps(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{})

	case *v1core.Secret:
		return m.client.Core().Secrets(m.Namespace).Delete(def.GetName(), &metav1.DeleteOptions{})

	default:
		return fmt.Errorf("object type '%v' unknown", def)
	}
}

// ParseJSON dynamically parses a json string
func ParseJSON(item string, keys ...string) (result string, err error) {
	result = fastjson.GetString([]byte(item), keys...)
	if result == "" {
		err = errors.New("failed to find pattern in json")
	}
	return
}

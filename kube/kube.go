package kube

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	// import all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// K8s represents a connection to kubernetes
type K8s struct {
	typed   kubernetes.Interface
	dynamic dynamic.Interface
	config  *rest.Config
	base    clientcmd.ClientConfig
	logger  *log.Entry
}

// NewClient populates a new connection
func NewClient(conf string) *K8s {
	var k8s K8s
	var err error

	if conf == "" {
		conf = clientcmd.RecommendedHomeFile
	}
	k8s.config, err = clientcmd.BuildConfigFromFlags("", conf)
	if err != nil {
		log.Fatal(err)
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	k8s.base = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	if k8s.typed, err = kubernetes.NewForConfig(k8s.config); err != nil {
		log.Fatal(err)
	}

	if k8s.dynamic, err = dynamic.NewForConfig(k8s.config); err != nil {
		log.Fatal(err)
	}

	return &k8s
}

// NewFakeClient returns a testing instance
func NewFakeClient() *K8s {
	scheme := runtime.NewScheme()
	return &K8s{
		typed:   kfake.NewSimpleClientset(),
		dynamic: dfake.NewSimpleDynamicClient(scheme),
	}
}

// FindPod finds a pod based on the namespace and the given label
func (k8s *K8s) FindPod(namespace, label string) (result string, err error) {
	pods, err := k8s.typed.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: label})
	if len(pods.Items) < 1 {
		return result, errors.New("no pods found")
	}
	return pods.Items[0].Name, err
}

// ForwardPod establishes a persistent connection to a remote pod
func (k8s *K8s) ForwardPod(name, namespace, local, remote string) chan struct{} {
	roundTripper, upgrader, err := spdy.RoundTripperFor(k8s.config)
	if err != nil {
		log.Fatal(err)
	}

	tillerName, err := k8s.FindPod(namespace, fmt.Sprintf("name=%s", name))
	if err != nil {
		log.Fatal(fmt.Errorf("can't find tiller: %s", err))
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", "kube-system", tillerName)
	hostIP := strings.TrimLeft(k8s.config.Host, "https://")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	// ports = local, remote
	forwarder, err := portforward.New(dialer, []string{fmt.Sprintf("%s:%s", local, remote)}, stopChan, readyChan, out, errOut)
	if err != nil {
		log.Fatal(err)
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

func (k8s *K8s) getPodLogs(namespace, name string) (string, error) {
	req := k8s.typed.CoreV1().Pods(namespace).GetLogs(name, &v1core.PodLogOptions{})
	readCloser, err := req.Stream()
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, readCloser)
	return buf.String(), err
}

func (k8s *K8s) waitPod(namespace, pod string, remove bool, timeout int64) error {
	// make a watcher to wait for this pod to be ready
	watch, err := k8s.typed.CoreV1().Pods(namespace).Watch(metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", pod), TimeoutSeconds: &timeout})
	if err != nil {
		return err
	}

	for event := range watch.ResultChan() {
		phase := event.Object.(*v1core.Pod).Status.Phase
		if phase == v1core.PodSucceeded {
			return k8s.getLogsAndDelete(namespace, pod)
		} else if phase == v1core.PodFailed || phase == v1core.PodUnknown {
			return k8s.getLogsAndDelete(namespace, pod)
		} else if phase == v1core.PodRunning {
			return nil
		}
	}

	return fmt.Errorf("something went wrong waiting for pod")
}

func (k8s *K8s) getLogsAndDelete(namespace, pod string) error {
	logs, err := k8s.getPodLogs(namespace, pod)
	if err != nil {
		return err
	}
	fmt.Println(logs)
	return k8s.typed.CoreV1().Pods(namespace).Delete(pod, &metav1.DeleteOptions{})
}

// FromConfigMap reads an entry from a ConfigMap
func (k8s *K8s) FromConfigMap(name, namespace, key string) (result string, err error) {
	cm, err := k8s.typed.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if cm == nil {
		return result, errors.New("failed to get configmap")
	} else if err != nil {
		return "", err
	}
	return cm.Data[key], nil
}

// FromSecret reads an entry from a Secret
func (k8s *K8s) FromSecret(name, namespace, key string) (result string, err error) {
	sec, err := k8s.typed.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if sec == nil {
		return result, errors.New("failed to get secret")
	} else if err != nil {
		return "", err
	}
	return string(sec.Data[key]), nil
}

// CreateNamespace tells the k8s api to make a namespace
func (k8s *K8s) CreateNamespace(name string) error {
	ns := &v1core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := k8s.typed.CoreV1().Namespaces().Create(ns)
	return err
}

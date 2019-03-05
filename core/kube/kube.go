package kube

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/valyala/fastjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type K8s struct {
	client kubernetes.Interface
	config *rest.Config
}

func NewK8s() *K8s {
	// Fetch in-cluster config, if err try local
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			panic(err)
		}
	}

	k := K8s{}
	k.client, err = kubernetes.NewForConfig(config)
	k.config = config
	if err != nil {
		panic(err)
	}

	return &k
}

func (k8s *K8s) FindPod(name, namespace string) (result string, err error) {
	pods, err := k8s.client.Core().Pods(namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("name=%s", name)})
	if len(pods.Items) < 1 {
		return result, errors.New("tiller not found")
	}
	return pods.Items[0].Name, err
}

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

func (k8s *K8s) FromConfigMap(name, namespace, key string) (result string, err error) {
	cm, err := k8s.client.Core().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if cm == nil {
		return result, errors.New("failed to get configmap")
	}
	return cm.Data[key], nil
}

func (k8s *K8s) FromSecret(name, namespace, key string) (result string, err error) {
	sec, err := k8s.client.Core().Secrets(namespace).Get(name, metav1.GetOptions{})
	if sec == nil {
		return result, errors.New("failed to get secret")
	}
	return string(sec.Data[key]), nil
}

func ParseJSON(item string, keys ...string) (result string, err error) {
	result = fastjson.GetString([]byte(item), keys...)
	if result == "" {
		err = errors.New("failed to find pattern in json")
	}
	return
}

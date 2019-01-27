package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
)

const remotePort = "44133"
const localPort = "44134"

type k8s struct {
	client kubernetes.Interface
	config *rest.Config
}

func newK8s() *k8s {
	// Fetch in-cluster config, if err try local.
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			panic(err)
		}
	}

	k := k8s{}
	k.client, err = kubernetes.NewForConfig(config)
	k.config = config
	if err != nil {
		panic(err)
	}

	return &k
}

// Helm represents a new helm client and connection to Tiller.
type Helm struct {
	client helm.Interface
	envset helm_env.EnvSettings
	tiller chan struct{}
}

func setupHelm() *Helm {
	tillerTunnelAddress := fmt.Sprintf("localhost:%s", localPort)
	hc := helm.NewClient(helm.Host(tillerTunnelAddress))
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")
	return &Helm{
		client: hc,
		envset: settings,
		tiller: forwardTiller(),
	}
}

func findTiller(namespace string, k8s *k8s) string {
	pods, err := k8s.client.Core().Pods(namespace).List(metav1.ListOptions{LabelSelector: "name=tiller"})
	if err != nil || len(pods.Items) != 1 {
		panic("Tiller not found.")
	}
	return pods.Items[0].Name
}

func forwardTiller() chan struct{} {
	k8s := newK8s()
	roundTripper, upgrader, err := spdy.RoundTripperFor(k8s.config)
	if err != nil {
		panic(err)
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", "kube-system", findTiller("kube-system", k8s))
	hostIP := strings.TrimLeft(k8s.config.Host, "https://")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forwarder, err := portforward.New(dialer, []string{localPort, remotePort}, stopChan, readyChan, out, errOut)
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

func downloadChart(location, version string, settings helm_env.EnvSettings) (string, error) {
	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Getters:  getter.All(settings),
	}
	if _, err := os.Stat(settings.Home.Archive()); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory.\n", settings.Home.Archive())
		os.MkdirAll(settings.Home.Archive(), 0744)
	}

	chart, _, err := dl.DownloadTo(location, version, settings.Home.Archive())
	return chart, err
}

func installChart(helmClient helm.Interface, settings helm_env.EnvSettings, chart Chart, values []byte) {
	name := fmt.Sprintf("%s/%s", chart.Repo, chart.Name)

	crt, _ := downloadChart(name, chart.Version, settings)
	requestedChart, _ := chartutil.Load(crt)
	chartutil.LoadRequirements(requestedChart)

	_, err := helmClient.InstallReleaseFromChart(
		requestedChart,
		chart.Namespace,
		helm.ReleaseName(chart.Release),
		helm.InstallWait(true),
		helm.InstallTimeout(300),
		helm.ValueOverrides(values),
		helm.InstallDryRun(false),
	)
	if err != nil {
		fmt.Println(err)
	}
}

func upgradeChart(helmClient helm.Interface, settings helm_env.EnvSettings, chart Chart, values []byte) {
	_, _ = url.ParseRequestURI(chart.Repo)
	name := fmt.Sprintf("%s/%s", chart.Repo, chart.Name)

	crt, _ := downloadChart(name, chart.Version, settings)

	_, err := helmClient.UpdateRelease(
		chart.Release,
		crt,
		helm.UpgradeTimeout(300),
		helm.UpdateValueOverrides(values),
		helm.UpgradeDryRun(false),
	)
	if err != nil {
		fmt.Println(err)
	}
}

func deleteChart(helmClient helm.Interface, release string) error {
	_, err := helmClient.DeleteRelease(
		release,
		helm.DeletePurge(true),
		helm.DeleteDryRun(false),
	)
	return err
}

func releaseStatus(helmClient helm.Interface, release string) (string, error) {
	out, err := helmClient.ReleaseStatus(release)
	if err != nil {
		return "", err
	}
	return out.GetInfo().Status.Code.String(), nil
}

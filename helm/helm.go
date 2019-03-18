package helm

import (
	"fmt"
	"net/url"
	"os"

	"github.com/monax/compass/kube"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
)

// Chart comprises the helm release
type Chart struct {
	Name       string `yaml:"name"`       // name of chart
	Repository string `yaml:"repository"` // chart repo
	Version    string `yaml:"version"`    // chart version
	Release    string `yaml:"release"`    // release name
	Timeout    int64  `yaml:"timeout"`    // install / upgrade wait time
}

// Bridge represents a helm client and open conn to tiller
type Bridge struct {
	client helm.Interface
	envset helm_env.EnvSettings
	tiller chan struct{}
}

// Setup creates a new connection to tiller
func Setup(namespace, port string) *Bridge {
	tillerTunnelAddress := fmt.Sprintf("localhost:%s", port)
	hc := helm.NewClient(helm.Host(tillerTunnelAddress), helm.ConnectTimeout(60))
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")
	k8s := kube.NewK8s()
	return &Bridge{
		client: hc,
		envset: settings,
		tiller: k8s.ForwardPod("tiller", namespace, port),
	}
}

// Close gracefully exits the connection to tiller
func (b *Bridge) Close() {
	close(b.tiller)
}

func downloadChart(location, version string, settings helm_env.EnvSettings) (string, error) {
	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Getters:  getter.All(settings),
	}
	if _, err := os.Stat(settings.Home.Archive()); os.IsNotExist(err) {
		fmt.Printf("creating directory: %s\n", settings.Home.Archive())
		os.MkdirAll(settings.Home.Archive(), 0744)
	}

	chart, _, err := dl.DownloadTo(location, version, settings.Home.Archive())
	return chart, err
}

// InstallChart deploys a helm chart
func (b *Bridge) InstallChart(chart Chart, namespace string, values []byte) error {
	name := fmt.Sprintf("%s/%s", chart.Repository, chart.Name)

	crt, err := downloadChart(name, chart.Version, b.envset)
	if err != nil {
		return err
	}

	requestedChart, err := chartutil.Load(crt)
	if err != nil {
		return err
	}

	chartutil.LoadRequirements(requestedChart)
	_, err = b.client.InstallReleaseFromChart(
		requestedChart,
		namespace,
		helm.ReleaseName(chart.Release),
		helm.InstallWait(true),
		helm.InstallTimeout(chart.Timeout),
		helm.ValueOverrides(values),
		helm.InstallDryRun(false),
	)
	if err != nil {
		return err
	}
	return nil
}

// UpgradeChart tells tiller to upgrade a helm chart
func (b *Bridge) UpgradeChart(chart Chart, values []byte) error {
	_, _ = url.ParseRequestURI(chart.Repository)
	name := fmt.Sprintf("%s/%s", chart.Repository, chart.Name)

	crt, err := downloadChart(name, chart.Version, b.envset)
	if err != nil {
		return err
	}

	_, err = b.client.UpdateRelease(
		chart.Release,
		crt,
		helm.UpgradeTimeout(chart.Timeout),
		helm.UpdateValueOverrides(values),
		helm.UpgradeDryRun(false),
	)
	if err != nil {
		return err
	}
	return nil
}

// DeleteRelease tells tiller to destroy a release
func (b *Bridge) DeleteRelease(release string) error {
	_, err := b.client.DeleteRelease(
		release,
		helm.DeletePurge(true),
		helm.DeleteTimeout(60),
		helm.DeleteDryRun(false),
	)
	return err
}

// ReleaseStatus returns the status of a release
func (b *Bridge) ReleaseStatus(release string) (string, error) {
	out, err := b.client.ReleaseStatus(release)
	if err != nil {
		return "", err
	}
	return out.GetInfo().Status.Code.String(), nil
}

// NewFakeBridge establishes a fake helm client
func NewFakeBridge() *Bridge {
	b := Bridge{}
	var client helm.FakeClient
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")
	b.client = client.Option()
	b.envset = settings
	return &b
}

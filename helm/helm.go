package helm

import (
	"fmt"
	"net/url"
	"os"

	"github.com/monax/compass/helm/kube"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
)

// Bridge represents a helm client and open conn to tiller
type Bridge struct {
	client helm.Interface
	envset helm_env.EnvSettings
	tiller chan struct{}
}

// Setup creates a new connection to tiller
func Setup(namespace, port string) *Bridge {
	tillerTunnelAddress := fmt.Sprintf("localhost:%s", port)
	hc := helm.NewClient(helm.Host(tillerTunnelAddress))
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

func installChart(helmClient helm.Interface, settings helm_env.EnvSettings, chart Chart, values []byte) error {
	name := fmt.Sprintf("%s/%s", chart.Repo, chart.Name)

	crt, err := downloadChart(name, chart.Version, settings)
	if err != nil {
		return err
	}

	requestedChart, err := chartutil.Load(crt)
	if err != nil {
		return err
	}

	chartutil.LoadRequirements(requestedChart)
	_, err = helmClient.InstallReleaseFromChart(
		requestedChart,
		chart.Namespace,
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

func upgradeChart(helmClient helm.Interface, settings helm_env.EnvSettings, chart Chart, values []byte) error {
	_, _ = url.ParseRequestURI(chart.Repo)
	name := fmt.Sprintf("%s/%s", chart.Repo, chart.Name)

	crt, err := downloadChart(name, chart.Version, settings)
	if err != nil {
		return err
	}

	_, err = helmClient.UpdateRelease(
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

package helm

import (
	"fmt"
	"os"
	"strconv"

	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	"github.com/phayes/freeport"
	log "github.com/sirupsen/logrus"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

var (
	helmLogger = log.WithFields(log.Fields{
		"kind": "helm",
	})
)

// Tiller represents a helm client and open connection to tiller
type Tiller struct {
	client helm.Interface
	envset helm_env.EnvSettings
	tiller chan struct{}
}

// NewClient creates a new connection to tiller
func NewClient(k8s *kube.K8s, namespace, remote string) *Tiller {
	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}
	local := strconv.Itoa(port)

	tillerTunnelAddress := fmt.Sprintf("localhost:%s", local)
	hl := helm.NewClient(helm.Host(tillerTunnelAddress), helm.ConnectTimeout(60))
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")

	return &Tiller{
		client: hl,
		envset: settings,
		tiller: k8s.ForwardPod("tiller", namespace, local, remote),
	}
}

// Close gracefully exits the connection to tiller
func (hl *Tiller) Close() {
	close(hl.tiller)
}

// Chart comprises the helm release
type Chart struct {
	Name      string `yaml:"name"`      // name of chart
	Version   string `yaml:"version"`   // chart version
	Release   string `yaml:"release"`   // release name
	Namespace string `yaml:"namespace"` // namespace
	Timeout   int64  `yaml:"timeout"`   // install / upgrade wait time
	Object    []byte
	*Tiller
}

// Lint validates the chart for required values
// some of which are parsed from values
func (c *Chart) Lint(key string, in *util.Values) error {
	c.Version = in.Cascade(c.Version, key, "version")
	if c.Namespace = in.Cascade(c.Namespace, key, "namespace"); c.Namespace == "" {
		return fmt.Errorf("namespace for %s is empty", key)
	}
	if c.Release = in.Cascade(c.Release, key, "release"); c.Release == "" {
		return fmt.Errorf("release for %s is empty", key)
	}
	if c.Name == "" {
		return fmt.Errorf("chart name required in the format repo/app")
	}
	return nil
}

// SetInput adds the templated values file
func (c *Chart) SetInput(obj []byte) {
	c.Object = obj
}

// GetInput gets the templated values file
func (c *Chart) GetInput() []byte {
	return c.Object
}

// Connect sets the established helm connection
func (c *Chart) Connect(bridge interface{}) {
	c.Tiller = bridge.(*Tiller)
}

func downloadChart(location, version string, settings helm_env.EnvSettings) (*chart.Chart, error) {
	if util.IsDir(location) {
		helmLogger.Infof("Using local chart: %s", location)
		return chartutil.LoadDir(location)
	}

	helmLogger.Infof("Downloading: %s", location)
	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Getters:  getter.All(settings),
	}
	if _, err := os.Stat(settings.Home.Archive()); os.IsNotExist(err) {
		fmt.Printf("Creating directory: %s\n", settings.Home.Archive())
		os.MkdirAll(settings.Home.Archive(), 0744)
	}

	chart, _, err := dl.DownloadTo(location, version, settings.Home.Archive())
	if err != nil {
		return nil, err
	}

	return chartutil.Load(chart)
}

// Status returns the status of a release
func (c *Chart) Status() (bool, error) {
	out, err := c.client.ReleaseStatus(c.Release)
	if err != nil || out == nil {
		return false, err
	}
	statusCode := out.GetInfo().Status.Code.String()
	if statusCode == "PENDING_INSTALL" {
		c.Delete()
	}
	return true, nil
}

// Install deploys a helm chart
func (c *Chart) Install() error {
	reqChart, err := downloadChart(c.Name, c.Version, c.envset)
	if err != nil {
		return err
	}

	helmLogger.Infof("Releasing: %s (%s)", c.Release, reqChart.GetMetadata().GetVersion())
	chartutil.LoadRequirements(reqChart)
	_, err = c.client.InstallReleaseFromChart(
		reqChart,
		c.Namespace,
		helm.ReleaseName(c.Release),
		helm.InstallWait(true),
		helm.InstallTimeout(c.Timeout),
		helm.ValueOverrides(c.Object),
		helm.InstallDryRun(false),
	)
	if err != nil {
		return err
	}
	return nil
}

// Upgrade tells tiller to upgrade a helm chart
func (c *Chart) Upgrade() error {
	reqChart, err := downloadChart(c.Name, c.Version, c.envset)
	if err != nil {
		return err
	}

	helmLogger.Infof("Releasing: %s (%s)", c.Release, reqChart.GetMetadata().GetVersion())
	_, err = c.client.UpdateReleaseFromChart(
		c.Release,
		reqChart,
		helm.UpgradeTimeout(c.Timeout),
		helm.UpdateValueOverrides(c.Object),
		helm.UpgradeDryRun(false),
	)
	if err != nil {
		return err
	}
	return nil
}

// Delete tells tiller to destroy a release
func (c *Chart) Delete() error {
	_, err := c.client.DeleteRelease(
		c.Release,
		helm.DeletePurge(true),
		helm.DeleteTimeout(60),
		helm.DeleteDryRun(false),
	)
	return err
}

// NewFakeClient establishes a fake helm client
func NewFakeClient() *Tiller {
	hl := Tiller{}
	var client helm.FakeClient
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")
	hl.client = client.Option()
	hl.envset = settings
	return &hl
}

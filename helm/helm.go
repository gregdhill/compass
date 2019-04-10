package helm

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"

	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	"github.com/phayes/freeport"
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
func Setup(k8s *kube.K8s, namespace, remote string) *Bridge {

	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}
	local := strconv.Itoa(port)

	tillerTunnelAddress := fmt.Sprintf("localhost:%s", local)
	hc := helm.NewClient(helm.Host(tillerTunnelAddress), helm.ConnectTimeout(60))
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")

	return &Bridge{
		client: hc,
		envset: settings,
		tiller: k8s.ForwardPod("tiller", namespace, local, remote),
	}
}

// Close gracefully exits the connection to tiller
func (b *Bridge) Close() {
	close(b.tiller)
}

// Chart comprises the helm release
type Chart struct {
	Name       string `yaml:"name"`       // name of chart
	Repository string `yaml:"repository"` // chart repository
	Version    string `yaml:"version"`    // chart version
	Release    string `yaml:"release"`    // release name
	Namespace  string `yaml:"namespace"`  // namespace
	Timeout    int64  `yaml:"timeout"`    // install / upgrade wait time
	Object     []byte
	*Bridge
}

// Lint validates the chart for required values
// some of which are parsed from values
func (c *Chart) Lint(key string, in *util.Values) error {
	if c.Namespace = in.Cascade(key, "namespace", c.Namespace); c.Namespace == "" {
		return fmt.Errorf("namespace for %s is empty", key)
	}
	if c.Release = in.Cascade(key, "release", c.Release); c.Release == "" {
		return fmt.Errorf("release for %s is empty", key)
	}
	c.Version = in.Cascade(key, "version", c.Version)
	return nil
}

// SetInput adds the templated values file
func (c *Chart) SetInput(obj []byte) {
	c.Object = obj
}

// Connect sets the established helm connection
func (c *Chart) Connect(bridge interface{}) {
	c.Bridge = bridge.(*Bridge)
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

// Install deploys a helm chart
func (c *Chart) Install() error {
	name := fmt.Sprintf("%s/%s", c.Repository, c.Name)

	crt, err := downloadChart(name, c.Version, c.envset)
	if err != nil {
		return err
	}

	requestedChart, err := chartutil.Load(crt)
	if err != nil {
		return err
	}

	chartutil.LoadRequirements(requestedChart)
	_, err = c.client.InstallReleaseFromChart(
		requestedChart,
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
	_, _ = url.ParseRequestURI(c.Repository)
	name := fmt.Sprintf("%s/%s", c.Repository, c.Name)

	crt, err := downloadChart(name, c.Version, c.envset)
	if err != nil {
		return err
	}

	_, err = c.client.UpdateRelease(
		c.Release,
		crt,
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

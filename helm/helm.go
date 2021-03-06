package helm

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	log "github.com/sirupsen/logrus"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
)

// Tiller represents a helm client and open connection to tiller
type Tiller struct {
	client helm.Interface
	envset helm_env.EnvSettings
	tiller chan struct{}
	logger *log.Entry
}

// NewClient creates a new connection to tiller
func NewClient(k8s *kube.K8s, conf, namespace, remote string) (*Tiller, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	if err = listener.Close(); err != nil {
		return nil, err
	}

	tillerTunnelAddress := listener.Addr().String()
	localPort := strings.Split(tillerTunnelAddress, ":")[1]
	hl := helm.NewClient(helm.Host(tillerTunnelAddress), helm.ConnectTimeout(60))

	var settings helm_env.EnvSettings
	if conf == "" {
		conf = helm_env.DefaultHelmHome
	}
	settings.Home = helmpath.Home(conf)

	return &Tiller{
		client: hl,
		envset: settings,
		tiller: k8s.ForwardPod("tiller", namespace, localPort, remote),
		logger: log.WithFields(log.Fields{
			"kind": "helm",
		}),
	}, nil
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
	c.Version = in.Cascade(c.Version, key, "chart_version")
	if c.Release = in.Cascade(c.Release, key, "release"); c.Release == "" {
		return fmt.Errorf("release for %s is empty", key)
	}
	if c.Namespace = in.Cascade(c.Namespace, key, "namespace"); c.Namespace == "" {
		return fmt.Errorf("namespace for %s is empty", key)
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

// Download a chart to the local cache
func (c *Chart) Download() (*chart.Chart, error) {
	if util.IsDir(c.Name) {
		c.logger.Infof("Using local chart: %s", c.Name)
		return chartutil.LoadDir(c.Name)
	}

	c.logger.Infof("Downloading: %s", c.Name)
	dl := downloader.ChartDownloader{
		HelmHome: c.envset.Home,
		Getters:  getter.All(c.envset),
	}
	if _, err := os.Stat(c.envset.Home.Archive()); os.IsNotExist(err) {
		c.logger.Infof("Creating directory: %s\n", c.envset.Home.Archive())
		err := os.MkdirAll(c.envset.Home.Archive(), 0744)
		if err != nil {
			return nil, err
		}
	}

	chart, _, err := dl.DownloadTo(c.Name, c.Version, c.envset.Home.Archive())
	if err != nil {
		return nil, err
	}

	return chartutil.Load(chart)
}

// Status returns the status of a release
// true if exists, else false
func (c *Chart) Status() (bool, error) {
	rs, err := c.client.ReleaseStatus(c.Release)
	if err != nil || rs == nil {
		// we can probably be smarter about this
		// but typically helm returns an error if
		// the release doesn't exist
		return false, err
	}

	// exists; check what state it's in
	statusCode := rs.GetInfo().Status.Code

	if statusCode == release.Status_PENDING_INSTALL {
		return false, c.Delete()
	} else if statusCode == release.Status_FAILED {
		rh, err := c.client.ReleaseHistory(c.Release)
		if err != nil {
			return false, err
		}
		// helm won't let us upgrade if the first release failed
		if releases := rh.GetReleases(); len(releases) <= 1 {
			return false, c.Delete()
		}
	}

	// TODO: check other statuses
	return true, nil
}

// InstallOrUpgrade deploys a helm chart
func (c *Chart) InstallOrUpgrade() error {
	exists, _ := c.Status()
	reqChart, err := c.Download()
	if err != nil {
		return err
	}

	c.logger.Infof("Releasing: %s (%s)", c.Release, reqChart.GetMetadata().GetVersion())
	if !exists {
		return c.Install(reqChart)
	}
	return c.Upgrade(reqChart)
}

// Install tells tiller to install a helm chart
func (c *Chart) Install(req *chart.Chart) error {
	_, err := c.client.InstallReleaseFromChart(
		req,
		c.Namespace,
		helm.ReleaseName(c.Release),
		helm.InstallWait(true),
		helm.InstallTimeout(c.Timeout),
		helm.ValueOverrides(c.Object),
		helm.InstallDryRun(false),
	)
	return err
}

// Upgrade tells tiller to upgrade a helm chart
func (c *Chart) Upgrade(req *chart.Chart) error {
	_, err := c.client.UpdateReleaseFromChart(
		c.Release,
		req,
		helm.UpgradeTimeout(c.Timeout),
		helm.UpdateValueOverrides(c.Object),
		helm.UpgradeDryRun(false),
	)
	return err
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
	var client helm.FakeClient
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")
	return &Tiller{
		client: client.Option(),
		envset: settings,
		logger: log.StandardLogger().WithField("kind", "helm"),
	}
}

package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
)

func newTestChart() Chart {
	return Chart{
		Name:       "burrow",
		Repository: "stable",
		Version:    "",
		Release:    "test-release",
		Namespace:  "test-namespace",
		Tiller:     NewFakeClient(),
	}
}

func TestDownloadChart(t *testing.T) {
	b := NewFakeClient()
	dl := downloader.ChartDownloader{
		HelmHome: b.envset.Home,
		Getters:  getter.All(b.envset),
	}

	_, err := downloadChart("fake/chart", "", b.envset)
	assert.Error(t, err, "repo fake not found")

	_, err = downloadChart("stable/burrow", "", b.envset)
	assert.NoError(t, err)

	url, _, _ := dl.ResolveChartVersion("stable/burrow", "1.0.0")
	assert.Equal(t, "https://kubernetes-charts.storage.googleapis.com/burrow-1.0.0.tgz", url.String())
}

func TestReleaseStatus(t *testing.T) {
	c := newTestChart()

	_, err := c.Status()
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = c.client.InstallRelease(c.Name, c.Namespace, helm.ReleaseName(c.Release))
	assert.NoError(t, err)
}

func TestDeleteChart(t *testing.T) {
	c := newTestChart()

	err := c.Delete()
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = c.client.InstallRelease(c.Name, c.Namespace, helm.ReleaseName(c.Release))
	assert.NoError(t, err)

	err = c.Delete()
	assert.NoError(t, err)
}

func TestInstallChart(t *testing.T) {
	c := newTestChart()
	c.Install()
	out, _ := c.Status()
	assert.Equal(t, true, out)
}

func TestUpgradeChart(t *testing.T) {
	c := newTestChart()

	_, err := c.Tiller.client.InstallRelease(c.Name, c.Namespace, helm.ReleaseName(c.Release), helm.InstallWait(true))
	assert.NoError(t, err)

	c.Upgrade()
	out, err := c.Status()
	assert.NoError(t, err)
	assert.Equal(t, true, out)
}

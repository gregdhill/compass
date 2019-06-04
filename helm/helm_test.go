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
		Name:      "stable/burrow",
		Version:   "",
		Release:   "test-release",
		Namespace: "test-namespace",
		Tiller:    NewFakeClient(),
	}
}

func TestDownloadChart(t *testing.T) {
	cli := NewFakeClient()

	dl := downloader.ChartDownloader{
		HelmHome: cli.envset.Home,
		Getters:  getter.All(cli.envset),
	}

	_, err := downloadChart("fake/chart", "", cli.envset)
	assert.Error(t, err, "repo fake not found")

	_, err = downloadChart("stable/burrow", "", cli.envset)
	assert.NoError(t, err)

	url, _, _ := dl.ResolveChartVersion("stable/burrow", "1.0.0")
	assert.Equal(t, "https://kubernetes-charts.storage.googleapis.com/burrow-1.0.0.tgz", url.String())
}

func TestReleaseStatus(t *testing.T) {
	chart := newTestChart()

	_, err := chart.Status()
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = chart.client.InstallRelease(chart.Name, chart.Namespace, helm.ReleaseName(chart.Release))
	assert.NoError(t, err)
}

func TestDeleteChart(t *testing.T) {
	chart := newTestChart()

	err := chart.Delete()
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = chart.client.InstallRelease(chart.Name, chart.Namespace, helm.ReleaseName(chart.Release))
	assert.NoError(t, err)

	err = chart.Delete()
	assert.NoError(t, err)
}

func TestInstallChart(t *testing.T) {
	chart := newTestChart()
	chart.Install()
	out, _ := chart.Status()
	assert.Equal(t, true, out)
}

func TestUpgradeChart(t *testing.T) {
	chart := newTestChart()

	_, err := chart.Tiller.client.InstallRelease(chart.Name, chart.Namespace, helm.ReleaseName(chart.Release), helm.InstallWait(true))
	assert.NoError(t, err)

	chart.Upgrade()
	out, err := chart.Status()
	assert.NoError(t, err)
	assert.Equal(t, true, out)
}

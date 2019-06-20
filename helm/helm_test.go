package helm

import (
	"fmt"
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

func TestDownload(t *testing.T) {
	t.Run("Charts", func(t *testing.T) {
		t.Run("NotExist", func(t *testing.T) {
			t.Parallel()
			chart := newTestChart()
			chart.Name = "fake/chart"
			_, err := chart.Download()
			assert.Error(t, err)
		})
		t.Run("Version", func(t *testing.T) {
			t.Parallel()
			chart := newTestChart()
			chart.Version = "1.0.0"
			downloaded, err := chart.Download()
			assert.NoError(t, err)
			assert.Equal(t, chart.Version, downloaded.GetMetadata().GetVersion())
		})
		t.Run("NoVersion", func(t *testing.T) {
			t.Parallel()
			chart := newTestChart()
			downloaded, err := chart.Download()
			assert.NoError(t, err)

			dl := downloader.ChartDownloader{
				HelmHome: chart.envset.Home,
				Getters:  getter.All(chart.envset),
			}

			url, _, err := dl.ResolveChartVersion(chart.Name, chart.Version)
			assert.NoError(t, err)
			actual := fmt.Sprintf("https://kubernetes-charts.storage.googleapis.com/burrow-%s.tgz", downloaded.GetMetadata().GetVersion())
			assert.Equal(t, url.String(), actual)

		})
	})
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
	chart.InstallOrUpgrade()
	out, _ := chart.Status()
	assert.Equal(t, true, out)
}

func TestUpgradeChart(t *testing.T) {
	chart := newTestChart()

	_, err := chart.Tiller.client.InstallRelease(chart.Name, chart.Namespace, helm.ReleaseName(chart.Release), helm.InstallWait(true))
	assert.NoError(t, err)

	chart.InstallOrUpgrade()
	out, err := chart.Status()
	assert.NoError(t, err)
	assert.Equal(t, true, out)
}

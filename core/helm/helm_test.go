package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
)

func TestDownloadChart(t *testing.T) {
	b := NewFakeBridge()
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
	b := NewFakeBridge()

	_, err := b.ReleaseStatus("test-release")
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = b.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	_, err = b.ReleaseStatus("test-release")
	assert.NoError(t, err)
}

func TestDeleteChart(t *testing.T) {
	b := NewFakeBridge()

	err := b.DeleteRelease("test-release")
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = b.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	err = b.DeleteRelease("test-release")
	assert.NoError(t, err)
}

func TestInstallChart(t *testing.T) {
	b := NewFakeBridge()
	c := Chart{
		Name:    "burrow",
		Repo:    "stable",
		Version: "",
		Release: "test-release",
	}

	b.InstallChart(c, nil)
	out, _ := b.ReleaseStatus(c.Release)
	assert.Equal(t, "DEPLOYED", out)
}

func TestUpgradeChart(t *testing.T) {
	b := NewFakeBridge()
	c := Chart{
		Name:    "burrow",
		Repo:    "stable",
		Version: "",
		Release: "test-release",
	}

	_, err := b.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	b.UpgradeChart(c, nil)
	out, _ := b.ReleaseStatus("test-release")
	assert.Equal(t, "DEPLOYED", out)
}

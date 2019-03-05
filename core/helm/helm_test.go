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
	hc := NewFakeBridge()
	dl := downloader.ChartDownloader{
		HelmHome: hc.envset.Home,
		Getters:  getter.All(hc.envset),
	}

	_, err := downloadChart("fake/chart", "", hc.envset)
	assert.Error(t, err, "repo fake not found")

	_, err = downloadChart("stable/burrow", "", hc.envset)
	assert.NoError(t, err)

	url, _, _ := dl.ResolveChartVersion("stable/burrow", "1.0.0")
	assert.Equal(t, "https://kubernetes-charts.storage.googleapis.com/burrow-1.0.0.tgz", url.String())
}

func TestReleaseStatus(t *testing.T) {
	hc := NewFakeBridge()

	_, err := ReleaseStatus(hc, "test-release")
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = hc.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	_, err = ReleaseStatus(hc, "test-release")
	assert.NoError(t, err)
}

func TestDeleteChart(t *testing.T) {
	hc := NewFakeBridge()

	err := DeleteChart(hc, "test-release")
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = hc.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	err = DeleteChart(hc, "test-release")
	assert.NoError(t, err)
}

func TestInstallChart(t *testing.T) {
	hc := NewFakeBridge()
	c := Chart{
		Name:    "burrow",
		Repo:    "stable",
		Version: "",
		Release: "test-release",
	}

	InstallChart(hc, c, nil)
	out, _ := ReleaseStatus(hc, c.Release)
	assert.Equal(t, "DEPLOYED", out)
}

func TestUpgradeChart(t *testing.T) {
	hc := NewFakeBridge()
	c := Chart{
		Name:    "burrow",
		Repo:    "stable",
		Version: "",
		Release: "test-release",
	}

	_, err := hc.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	UpgradeChart(hc, c, nil)
	out, _ := ReleaseStatus(hc, "test-release")
	assert.Equal(t, "DEPLOYED", out)
}

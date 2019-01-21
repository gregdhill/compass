package main

import (
	"os"
	"testing"

	"gotest.tools/assert"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
)

func newTestHelm() *Helm {
	hc := Helm{}
	var client helm.FakeClient
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")
	hc.client = client.Option()
	hc.envset = settings
	return &hc
}

func TestDeleteChart(t *testing.T) {
	hc := newTestHelm()

	err := deleteChart(hc.client, "test-release")
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = hc.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NilError(t, err)

	err = deleteChart(hc.client, "test-release")
	assert.NilError(t, err)
}

func TestReleaseStatus(t *testing.T) {
	hc := newTestHelm()

	_, err := releaseStatus(hc.client, "test-release")
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = hc.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NilError(t, err)

	_, err = releaseStatus(hc.client, "test-release")
	assert.NilError(t, err)
}

func TestDownloadChart(t *testing.T) {
	hc := newTestHelm()
	dl := downloader.ChartDownloader{
		HelmHome: hc.envset.Home,
		Getters:  getter.All(hc.envset),
	}

	_, err := downloadChart("fake/chart", "", hc.envset)
	assert.Error(t, err, "repo fake not found")

	_, err = downloadChart("stable/burrow", "", hc.envset)
	assert.NilError(t, err)

	url, _, _ := dl.ResolveChartVersion("stable/burrow", "1.0.0")
	assert.Equal(t, url.String(), "https://kubernetes-charts.storage.googleapis.com/burrow-1.0.0.tgz")
}

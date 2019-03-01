package helm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
)

func newTestHelm() *Bridge {
	hc := Bridge{}
	var client helm.FakeClient
	var settings helm_env.EnvSettings
	settings.Home = helmpath.Home(os.Getenv("HOME") + "/.helm")
	hc.client = client.Option()
	hc.envset = settings
	return &hc
}

func newTestK8s() *k8s {
	k := k8s{}
	k.client = fake.NewSimpleClientset()
	return &k
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
	assert.NoError(t, err)

	url, _, _ := dl.ResolveChartVersion("stable/burrow", "1.0.0")
	assert.Equal(t, "https://kubernetes-charts.storage.googleapis.com/burrow-1.0.0.tgz", url.String())
}

func TestReleaseStatus(t *testing.T) {
	hc := newTestHelm()

	_, err := releaseStatus(hc.client, "test-release")
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = hc.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	_, err = releaseStatus(hc.client, "test-release")
	assert.NoError(t, err)
}

func TestDeleteChart(t *testing.T) {
	hc := newTestHelm()

	err := deleteChart(hc.client, "test-release")
	assert.Error(t, err, "release: \"test-release\" not found")

	_, err = hc.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	err = deleteChart(hc.client, "test-release")
	assert.NoError(t, err)
}

func TestInstallChart(t *testing.T) {
	hc := newTestHelm()
	c := Chart{
		Name:    "burrow",
		Repo:    "stable",
		Version: "",
		Release: "test-release",
	}

	installChart(hc.client, hc.envset, c, nil)
	out, _ := releaseStatus(hc.client, c.Release)
	assert.Equal(t, "DEPLOYED", out)
}

func TestUpgradeChart(t *testing.T) {
	hc := newTestHelm()
	c := Chart{
		Name:    "burrow",
		Repo:    "stable",
		Version: "",
		Release: "test-release",
	}

	_, err := hc.client.InstallRelease("test-chart", "test-namespace", helm.ReleaseName("test-release"))
	assert.NoError(t, err)

	upgradeChart(hc.client, hc.envset, c, nil)
	out, _ := releaseStatus(hc.client, "test-release")
	assert.Equal(t, "DEPLOYED", out)
}

func TestFindTiller(t *testing.T) {
	k8s := newTestK8s()

	assert.Panics(t, func() { findTiller("kube-system", k8s) })

	n := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}
	_, err := k8s.client.Core().Namespaces().Create(n)
	if err != nil {
		t.Errorf("Error injecting namespace into fake client: %v", err)
	}

	p := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "tiller-test", Labels: map[string]string{"name": "tiller"}}}
	_, err = k8s.client.Core().Pods("kube-system").Create(p)
	if err != nil {
		t.Errorf("Error injecting pod into fake client: %v", err)
	}

	assert.Equal(t, "tiller-test", findTiller("kube-system", k8s))
}

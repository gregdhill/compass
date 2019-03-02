package helm

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/monax/compass/helm/docker"
	"github.com/monax/compass/helm/kube"
)

// Generate renders the given values template
func Generate(name string, data, out *[]byte, values map[string]string) {
	k8s := kube.NewK8s()

	funcMap := template.FuncMap{
		"readEnv":       os.Getenv,
		"getDigest":     docker.GetImageHash,
		"getAuth":       docker.GetAuthToken,
		"fromConfigMap": k8s.FromConfigMap,
		"parseJSON":     kube.ParseJSON,
	}

	t, err := template.New(name).Funcs(funcMap).Parse(string(*data))
	if err != nil {
		log.Fatalf("failed to render %s : %s\n", name, err)
	}

	buf := new(bytes.Buffer)
	err = t.Execute(buf, values)
	if err != nil {
		log.Fatalf("failed to render %s : %s\n", name, err)
	}
	*out = append(*out, buf.Bytes()...)
}

func Extrapolate(tpl string, values map[string]string) map[string]string {
	if tpl == "" {
		return values
	}
	data, err := ioutil.ReadFile(tpl)
	if err != nil {
		log.Fatalf("couldn't read from %s\n", tpl)
	}
	var out []byte
	Generate(tpl, &data, &out, values)
	MergeVals(values, LoadVals(tpl, out))
	return values
}

func shellVars(vals map[string]string) []string {
	envs := make([]string, len(vals))
	for key, value := range vals {
		envs = append(envs, fmt.Sprintf("%s=%s", key, value))
	}
	return envs
}

func shellJobs(values []string, jobs []string, verbose bool) error {
	for _, command := range jobs {
		log.Printf("running job: %s\n", command)
		args := strings.Fields(command)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(values, os.Environ()...)
		stdout, err := cmd.Output()
		if verbose && stdout != nil {
			fmt.Println(string(stdout))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func checkRequires(values map[string]string, reqs []string) error {
	for _, r := range reqs {
		if _, exists := values[r]; !exists {
			return errors.New("requirement not met")
		}
	}
	return nil
}

func cpVals(prev map[string]string) map[string]string {
	// copy values from main for individual chart
	values := make(map[string]string, len(prev))
	for k, v := range prev {
		values[k] = v
	}
	return values
}

// Remove deletes the chart once its dependencies have been met
func (chart *Chart) Remove(helm *Bridge, key string, values map[string]string, verbose bool, deps *Depends) error {
	defer deps.Complete(chart.Depends...)

	err := checkRequires(values, chart.Requires)
	if err != nil {
		return err
	}

	deps.Wait(key)
	log.Printf("deleting %s\n", chart.Release)
	return deleteChart(helm.client, chart.Release)
}

// Make creates the chart once its dependencies have been met
func (chart *Chart) Make(helm *Bridge, key string, main map[string]string, verbose bool, deps *Depends) error {
	defer deps.Complete(key)

	_, err := releaseStatus(helm.client, chart.Release)
	if err == nil && chart.Abandon {
		return errors.New("chart already installed")
	}

	values := cpVals(main)
	MergeVals(values, LoadVals(chart.Values, nil))
	MergeVals(values, map[string]string{"namespace": chart.Namespace})
	MergeVals(values, map[string]string{"release": chart.Release})

	err = checkRequires(values, chart.Requires)
	if err != nil {
		return err
	}

	deps.Wait(chart.Depends...)

	shellJobs(shellVars(values), chart.Jobs.Before, verbose)
	defer shellJobs(shellVars(values), chart.Jobs.After, verbose)

	var out []byte
	for _, temp := range chart.Templates {
		data, read := ioutil.ReadFile(temp)
		if read != nil {
			panic(read)
		}
		Generate(temp, &data, &out, values)
	}

	if verbose {
		fmt.Println(string(out))
	}

	status, err := releaseStatus(helm.client, chart.Release)
	if status == "PENDING_INSTALL" || err != nil {
		if err == nil {
			log.Printf("deleting release: %s\n", chart.Release)
			deleteChart(helm.client, chart.Release)
		}
		log.Printf("installing release: %s\n", chart.Release)
		err := installChart(helm.client, helm.envset, *chart, out)
		if err != nil {
			log.Fatalf("failed to install %s : %s\n", chart.Release, err)
		}
		log.Printf("release %s installed\n", chart.Release)
		return nil
	}

	log.Printf("upgrading release: %s\n", chart.Release)
	upgradeChart(helm.client, helm.envset, *chart, out)
	if err != nil {
		log.Fatalf("failed to install %s : %s\n", chart.Release, err)
	}
	log.Printf("release upgraded: %s\n", chart.Release)
	return nil
}

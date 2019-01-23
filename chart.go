package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func deleteDep(index string, deps []string) []string {
	for i, j := range deps {
		if j == index {
			deps = append(deps[:i], deps[i+1:]...)
		}
	}
	return deps
}

func render(out *[]byte, file string, values map[string]string) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}

	t, err := template.New("template").Parse(string(data))
	if err != nil {
		panic(err)
	}

	buf := new(bytes.Buffer)
	err = t.Execute(buf, values)
	if err != nil {
		panic(err)
	}
	*out = append(*out, buf.Bytes()...)
}

func shellVars(vals map[string]string) []string {
	envs := make([]string, len(vals))
	for key, value := range vals {
		envs = append(envs, fmt.Sprintf("%s=%s", key, value))
	}
	return envs
}

func shellJobs(chart Chart, values []string, jobs []string) {
	for _, command := range jobs {
		fmt.Printf("Running job: %s\n", command)
		args := strings.Fields(command)
		cmd := exec.Command(os.Getenv("SHELL"), args...)
		cmd.Env = values
		cmd.Env = append(cmd.Env, fmt.Sprintf("namespace=%s", chart.Namespace))
		cmd.Env = append(cmd.Env, fmt.Sprintf("release=%s", chart.Release))
		err := cmd.Run()
		if err != nil {
			panic(err)
		}
	}
}

func newChart(key string, helm Helm, chart Chart, values map[string]string, finished chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() { finished <- key }()

	_, err := releaseStatus(helm.client, chart.Release)
	if err == nil && chart.Abandon {
		return
	}

	mergeVals(values, loadVals(chart.Values))
	reqs := chart.Requires
	for _, r := range reqs {
		if _, exists := values[r]; !exists {
			return
		}
	}

	deps := chart.Depends
	for len(deps) > 0 {
		dep := <-finished
		finished <- dep
		deps = deleteDep(dep, deps)
	}

	shellJobs(chart, shellVars(values), chart.Jobs.Before)
	defer shellJobs(chart, shellVars(values), chart.Jobs.After)

	var out []byte
	for _, temp := range chart.Templates {
		render(&out, temp, values)
	}

	status, err := releaseStatus(helm.client, chart.Release)

	if status == "PENDING_INSTALL" || err != nil {
		if err == nil {
			fmt.Printf("Deleting release %s.\n", chart.Release)
			deleteChart(helm.client, chart.Release)
		}
		fmt.Printf("Installing release %s.\n", chart.Release)
		installChart(helm.client, helm.envset, chart.Release, chart.Namespace, chart.Repo, chart.Name, out)
		fmt.Printf("Release %s installed.\n", chart.Release)
		return
	}

	fmt.Printf("Upgrading release %s.\n", chart.Release)
	upgradeChart(helm.client, helm.envset, chart.Release, chart.Repo, chart.Name, out)
	fmt.Printf("Release %s upgraded.\n", chart.Release)
}

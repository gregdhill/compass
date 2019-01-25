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

var lock sync.Mutex

func deleteDep(index string, deps []string) []string {
	for i, j := range deps {
		if j == index {
			deps = append(deps[:i], deps[i+1:]...)
		}
	}
	return deps
}

func generate(data, out *[]byte, values map[string]string) {
	funcMap := template.FuncMap{
		"digest": dockerHash,
		"remove": removePattern,
	}

	t, err := template.New("chart").Funcs(funcMap).Parse(string(*data))
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

func shellJobs(values []string, jobs []string) {
	for _, command := range jobs {
		fmt.Printf("Running job: %s\n", command)
		args := strings.Fields(command)
		cmd := exec.Command(os.Getenv("SHELL"), args...)
		cmd.Env = values
		err := cmd.Run()
		if err != nil {
			panic(err)
		}
	}
}

func newChart(key string, helm Helm, chart Chart, values map[string]string, finished chan string, wg *sync.WaitGroup, plan bool) {
	defer wg.Done()
	defer func() { finished <- key }()

	_, err := releaseStatus(helm.client, chart.Release)
	if err == nil && chart.Abandon {
		return
	}

	lock.Lock()
	mergeVals(values, loadVals(chart.Values, nil))
	mergeVals(values, map[string]string{"namespace": chart.Namespace})
	mergeVals(values, map[string]string{"release": chart.Release})
	lock.Unlock()

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

	shellJobs(shellVars(values), chart.Jobs.Before)
	defer shellJobs(shellVars(values), chart.Jobs.After)

	var out []byte
	for _, temp := range chart.Templates {
		data, read := ioutil.ReadFile(temp)
		if read != nil {
			panic(read)
		}
		generate(&data, &out, values)
	}

	if plan {
		fmt.Println(string(out))
		return
	}

	status, err := releaseStatus(helm.client, chart.Release)

	if status == "PENDING_INSTALL" || err != nil {
		if err == nil {
			fmt.Printf("Deleting release %s.\n", chart.Release)
			deleteChart(helm.client, chart.Release)
		}
		fmt.Printf("Installing release %s.\n", chart.Release)
		installChart(helm.client, helm.envset, chart.Release, chart.Namespace, chart.Repo, chart.Name, chart.Version, out)
		fmt.Printf("Release %s installed.\n", chart.Release)
		return
	}

	fmt.Printf("Upgrading release %s.\n", chart.Release)
	upgradeChart(helm.client, helm.envset, chart.Release, chart.Repo, chart.Name, chart.Version, out)
	fmt.Printf("Release %s upgraded.\n", chart.Release)
}

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

func render(file string, values map[interface{}]interface{}) []byte {
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
	return buf.Bytes()
}

func postDeploy(chart Chart, values []string, deployed error, finished chan string, wg sync.WaitGroup) {
	defer func() { finished <- chart.Release }()
	defer wg.Done()

	if deployed == nil && chart.Abandon {
		return
	}

	for _, command := range chart.Jobs {
		fmt.Printf("Running job: %s\n", command)
		args := strings.Fields(command)
		// env := os.Environ()
		// env = append(env, values...)

		cmd := exec.Command(os.Getenv("SHELL"), args...)
		cmd.Env = values
		err := cmd.Run()
		if err != nil {
			panic(err)
		}
	}
}

func newChart(helm Helm, chart Chart, values map[interface{}]interface{}, finished chan string, wg sync.WaitGroup) {
	_, deployed := releaseStatus(helm.client, chart.Release)
	defer postDeploy(chart, bashVars(values), deployed, finished, wg)

	_, err := releaseStatus(helm.client, chart.Release)
	if err == nil && chart.Abandon {
		return
	}

	deps := chart.Depends
	for len(deps) > 0 {
		dep := <-finished
		finished <- dep
		deps = deleteDep(dep, deps)
	}

	var out []byte
	mergeVals(values, loadVals(chart.Values))
	if chart.Template != "" {
		out = render(chart.Template, values)
	}

	status, err := releaseStatus(helm.client, chart.Release)

	if status == "PENDING_INSTALL" || err != nil {
		if err == nil {
			fmt.Printf("Deleting release %s.\n", chart.Release)
			deleteChart(helm.client, chart.Release)
		}
		fmt.Printf("Installing release %s.\n", chart.Release)
		installChart(helm.client, helm.envset, chart.Release, chart.Namespace, chart.Repo, chart.Name, out)
		return
	}

	fmt.Printf("Upgrading release %s.\n", chart.Release)
	upgradeChart(helm.client, helm.envset, chart.Release, chart.Repo, chart.Name, out)
}

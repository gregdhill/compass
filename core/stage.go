package core

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

	"github.com/monax/compass/docker"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
)

// Jobs represent any shell scripts
type Jobs struct {
	Before []string `yaml:"before"`
	After  []string `yaml:"after"`
}

// Stage represents a single part of the deployment pipeline
type Stage struct {
	helm.Chart `yaml:",inline"`
	Abandon    bool     `yaml:"abandon"`   // install only
	Values     string   `yaml:"values"`    // env specific values
	Requires   []string `yaml:"requires"`  // env requirements
	Depends    []string `yaml:"depends"`   // dependencies
	Jobs       Jobs     `yaml:"jobs"`      // bash jobs
	Templates  []string `yaml:"templates"` // templates
}

// Generate renders the given values template
func Generate(name string, in, out *[]byte, values Values) error {
	k8s := kube.NewK8s()

	funcMap := template.FuncMap{
		"readEnv":       os.Getenv,
		"getDigest":     docker.GetImageHash,
		"getAuth":       docker.GetAuthToken,
		"fromConfigMap": k8s.FromConfigMap,
		"fromSecret":    k8s.FromSecret,
		"parseJSON":     kube.ParseJSON,
	}

	t, err := template.New(name).Funcs(funcMap).Parse(string(*in))
	if err != nil {
		return fmt.Errorf("failed to render %s: %s", name, err)
	}

	buf := new(bytes.Buffer)
	err = t.Execute(buf, values)
	if err != nil {
		return fmt.Errorf("failed to render %s: %s", name, err)
	}
	*out = append(*out, buf.Bytes()...)
	return nil
}

func shellJobs(values []string, jobs []string, verbose bool) {
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
			panic(err)
		}
	}
}

func checkRequires(values map[string]string, reqs []string) error {
	for _, r := range reqs {
		if _, exists := values[r]; !exists {
			return errors.New("requirement not met")
		}
	}
	return nil
}

// Destroy deletes the chart once its dependencies have been met
func (stage *Stage) Destroy(conn *helm.Bridge, key string, values map[string]string, verbose bool, deps *Depends) error {
	defer deps.Complete(stage.Depends...)

	err := checkRequires(values, stage.Requires)
	if err != nil {
		return err
	}

	deps.Wait(key)
	log.Printf("deleting %s\n", stage.Release)
	return conn.DeleteRelease(stage.Release)
}

// Create deploys the chart once its dependencies have been met
func (stage *Stage) Create(conn *helm.Bridge, key string, global Values, verbose bool, deps *Depends) error {
	defer deps.Complete(key)

	_, err := conn.ReleaseStatus(stage.Release)
	if err == nil && stage.Abandon {
		return errors.New("chart already installed")
	}

	local := global.Duplicate()
	local.Render(stage.Values)

	err = checkRequires(local, stage.Requires)
	if err != nil {
		return err
	}

	deps.Wait(stage.Depends...)

	shellJobs(local.ToSlice(), stage.Jobs.Before, verbose)
	defer shellJobs(local.ToSlice(), stage.Jobs.After, verbose)

	var out []byte
	for _, temp := range stage.Templates {
		data, read := ioutil.ReadFile(temp)
		if read != nil {
			panic(read)
		}
		err = Generate(temp, &data, &out, local)
		if err != nil {
			panic(err)
		}
	}

	if verbose {
		fmt.Println(string(out))
	}

	status, err := conn.ReleaseStatus(stage.Release)
	if status == "PENDING_INSTALL" || err != nil {
		if err == nil {
			log.Printf("deleting release: %s\n", stage.Release)
			conn.DeleteRelease(stage.Release)
		}
		log.Printf("installing release: %s\n", stage.Release)
		err := conn.InstallChart(stage.Chart, out)
		if err != nil {
			log.Fatalf("failed to install %s : %s\n", stage.Release, err)
		}
		log.Printf("release %s installed\n", stage.Release)
		return nil
	}

	log.Printf("upgrading release: %s\n", stage.Release)
	conn.UpgradeChart(stage.Chart, out)
	if err != nil {
		log.Fatalf("failed to install %s : %s\n", stage.Release, err)
	}
	log.Printf("release upgraded: %s\n", stage.Release)
	return nil
}

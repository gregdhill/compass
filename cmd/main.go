package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/monax/compass/core"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var (
	spec       string
	templates  []string
	values     map[string]string
	destroy    bool
	export     bool
	force      bool
	verbose    bool
	tillerName string
	tillerPort string
	until      string
	namespace  string
)

var rootCmd = &cobra.Command{
	Use:   "compass",
	Short: "Kubernetes & Helm",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		k8s := kube.NewK8s()
		vals := util.Values(values) // explicit cli inputs

		// additional template files
		for _, i := range templates {
			if err = vals.FromTemplate(i, func(name string, input util.Values) ([]byte, error) {
				return core.Template(name, input, k8s)
			}); err != nil {
				return fmt.Errorf("couldn't attach import %s: %v", i, err)
			}
		}

		if export {
			valOut, err := json.Marshal(vals)
			if err != nil {
				return fmt.Errorf("couldn't marshal values: %v", err)
			}
			fmt.Println(string(valOut))
			return nil
		}

		values = vals
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		// populate workflow with stages
		workflow := core.Stages{}
		if spec != "" {
			var data []byte
			if data, err = ioutil.ReadFile(spec); err != nil {
				return err
			}
			if err = yaml.Unmarshal([]byte(data), &workflow); err != nil {
				return err
			}
		}

		_, closer := workflow.Connect(tillerName, tillerPort)
		defer closer()

		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			closer()
			os.Exit(1)
		}()

		if spec == "" && !export {
			return fmt.Errorf("no workflow file provided as argument")
		} else if export {
			return nil
		}

		if err = workflow.Lint(values); err != nil {
			return err
		}

		force := force
		verbose := verbose
		if len(workflow) == 0 {
			return fmt.Errorf("nothing to run")
		}

		// reverse workflow
		if destroy {
			workflow.Destroy(values, force, verbose)
			return nil
		}

		// stop at desired key
		if until != "" {
			workflow.Until(values, force, verbose, until)
			return nil
		}

		// run full workflow
		workflow.Run(values, force, verbose)
		return nil
	},
}

var kubeCmd = &cobra.Command{
	Use:   "kube",
	Short: "Template and deploy given Kubernetes specification.",
	RunE: func(cmd *cobra.Command, args []string) error {
		k8s := kube.NewK8s()

		out, err := core.Template(spec, values, k8s)
		if err != nil {
			return err
		}

		man := kube.Manifest{
			Timeout:   300,
			Object:    out,
			Namespace: namespace,
			K8s:       kube.NewK8s(),
		}

		if err = man.Install(); err != nil {
			return err
		}

		log.Println("Deployed successfully")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&spec, "spec", "s", "", "YAML formatted pipeline file or Kubernetes spec")
	rootCmd.PersistentFlags().StringArrayVarP(&templates, "template", "t", nil, "YAML files with key:value mappings")
	rootCmd.PersistentFlags().StringToStringVar(&values, "value", nil, "Extra values to append to the pipeline")
	rootCmd.PersistentFlags().BoolVarP(&destroy, "destroy", "d", false, "Purge all releases, top-down")
	rootCmd.PersistentFlags().BoolVarP(&export, "export", "e", false, "Render JSON marshalled values")
	rootCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "Force install / upgrade / delete")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose debug information")
	rootCmd.Flags().StringVarP(&tillerName, "tillerName", "n", "kube-system", "Namespace to search for Tiller")
	rootCmd.Flags().StringVarP(&tillerPort, "tillerPort", "p", "44134", "Port to connect on Tiller")
	rootCmd.Flags().StringVarP(&until, "until", "u", "", "Deploy stage and dependencies")
	kubeCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to deploy to")
	rootCmd.AddCommand(kubeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

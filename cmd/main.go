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
	k8s        *kube.K8s
	templates  []string
	values     map[string]string
	destroy    bool
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
	Long:  `Deploy a templated pipeline or install a single manifest. If no command given, output values as JSON.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		k8s = kube.NewClient()
		vals := util.Values(values) // explicit cli inputs

		// additional template files
		for _, i := range templates {
			if err = vals.FromTemplate(i, func(name string, input util.Values) ([]byte, error) {
				return core.Template(name, input, k8s)
			}); err != nil {
				return fmt.Errorf("couldn't attach import %s: %v", i, err)
			}
		}

		values = vals
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if len(values) == 0 {
			return fmt.Errorf("no values supplied")
		}
		valOut, err := json.Marshal(values)
		if err != nil {
			return fmt.Errorf("couldn't marshal values: %v", err)
		}
		fmt.Println(string(valOut))
		return nil
	},
}

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Run the given workflow",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		spec := args[0]

		// populate workflow with stages
		workflow := core.Stages{}
		var data []byte
		if data, err = ioutil.ReadFile(spec); err != nil {
			return err
		}
		if err = yaml.Unmarshal([]byte(data), &workflow); err != nil {
			return err
		}

		closer, err := workflow.Connect(k8s, values, tillerName, tillerPort)
		defer closer()
		if err != nil {
			return err
		}

		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			os.Exit(1)
		}()

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
	Short: "Template and deploy given kubernetes spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spec := args[0]
		k8s := kube.NewClient()
		out, err := core.Template(spec, values, k8s)
		if err != nil {
			return err
		}

		man := kube.Manifest{
			Timeout:   300,
			Object:    out,
			Namespace: namespace,
			K8s:       k8s,
		}

		if err = man.Install(); err != nil {
			return err
		}

		log.Println("Deployed successfully")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringArrayVarP(&templates, "template", "t", nil, "YAML files with key:value mappings")
	rootCmd.PersistentFlags().StringToStringVar(&values, "value", nil, "extra values to append to the pipeline")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show verbose debug information")

	rootCmd.Flags().BoolVarP(&destroy, "destroy", "d", false, "purge all stages, top-down")
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "force install / upgrade / delete")
	flowCmd.Flags().StringVarP(&tillerName, "tillerName", "n", "kube-system", "namespace to search for Tiller")
	flowCmd.Flags().StringVarP(&tillerPort, "tillerPort", "p", "44134", "port to connect on Tiller")
	flowCmd.Flags().StringVarP(&until, "until", "u", "", "deploy stage and dependencies")
	rootCmd.AddCommand(flowCmd)

	kubeCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace to deploy to")
	rootCmd.AddCommand(kubeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

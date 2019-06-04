package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/monax/compass/core"
	"github.com/monax/compass/docker"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var (
	k8s        *kube.K8s
	templates  []string
	values     map[string]string
	builds     map[string]string
	tags       map[string]string
	destroy    bool
	force      bool
	buildCtx   string
	tillerName string
	tillerPort string
	until      string
	namespace  string
)

var rootCmd = &cobra.Command{
	Use:          "compass",
	Short:        "Kubernetes & Helm",
	Long:         `Deploy a templated pipeline or install a single manifest. If no command given, output values as JSON.`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		k8s = kube.NewClient()
		if values == nil {
			values = make(map[string]string)
		}
		vals := util.Values(values) // explicit cli inputs

		// additional template files
		for _, i := range templates {
			if err = vals.FromTemplate(i, func(name string, input util.Values) ([]byte, error) {
				return core.Render(name, input, k8s)
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

var runCmd = &cobra.Command{
	Use:          "run",
	Short:        "Run the given workflow",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		spec := args[0]

		// do builds and fetch tags
		ctx := context.Background()
		shas := make(map[string]string, len(builds)+len(tags))
		for k, v := range builds {
			shas[k], err = docker.BuildAndPush(ctx, buildCtx, v)
			if err != nil {
				return err
			}
		}
		for k, v := range tags {
			shas[k], err = docker.GetImageHash(v)
			if err != nil {
				return err
			}
		}

		// we want those digests before we
		// template the main workflow
		genVals := util.Values(values)
		genVals.Append(shas)

		// populate workflow with stages
		workflow := core.Stages{}
		var data []byte
		if data, err = core.Render(spec, genVals, k8s); err != nil {
			return err
		}
		if err = yaml.Unmarshal([]byte(data), &workflow); err != nil {
			return err
		}

		closer, err := workflow.Connect(k8s, genVals, tillerName, tillerPort)
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

		if err = workflow.Lint(genVals); err != nil {
			return err
		}

		force := force
		if len(workflow) == 0 {
			return fmt.Errorf("nothing to run")
		}

		// reverse workflow
		if destroy {
			workflow.Destroy(genVals, force)
			return nil
		}

		// stop at desired key
		if until != "" {
			workflow.Until(genVals, force, until)
			return nil
		}

		// run full workflow
		workflow.Run(genVals, force)
		return nil
	},
}

var kubeCmd = &cobra.Command{
	Use:   "kube",
	Short: "Template and deploy given kubernetes spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spec := args[0]
		out, err := core.Render(spec, values, k8s)
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

		log.Info("Deployed successfully")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringArrayVarP(&templates, "template", "t", nil, "file with key:value mappings (YAML)")
	rootCmd.PersistentFlags().StringToStringVar(&values, "value", nil, "explicit key:value pairs")
	rootCmd.PersistentFlags().StringToStringVar(&builds, "build", nil, "build specified dockerfile")
	rootCmd.PersistentFlags().StringToStringVar(&tags, "tag", nil, "get digest of image")

	runCmd.Flags().StringVarP(&buildCtx, "context", "c", ".", "context for building and packaging")
	runCmd.Flags().BoolVarP(&destroy, "destroy", "d", false, "purge all stages, top-down")
	runCmd.Flags().BoolVarP(&force, "force", "f", false, "force install / upgrade / delete")
	runCmd.Flags().StringVarP(&tillerName, "tillerName", "n", "kube-system", "namespace to search for Tiller")
	runCmd.Flags().StringVarP(&tillerPort, "tillerPort", "p", "44134", "port to connect on Tiller")
	runCmd.Flags().StringVarP(&until, "until", "u", "", "only deploy stage and dependencies")
	rootCmd.AddCommand(runCmd)

	kubeCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace to deploy")
	kubeCmd.MarkFlagRequired("namespace")
	rootCmd.AddCommand(kubeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

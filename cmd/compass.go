package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/monax/compass/core"
	"github.com/monax/compass/core/schema"
	"github.com/monax/compass/docker"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var (
	k8s          *kube.K8s
	templates    []string
	inValues     map[string]string
	outValues    util.Values
	builds       map[string]string
	tags         map[string]string
	destroy      bool
	force        bool
	buildCtx     string
	tillerName   string
	tillerPort   string
	until        string
	namespace    string
	kubeConfig   string
	helmConfig   string
	shortVersion bool
	toEnv        bool
)

var rootCmd = &cobra.Command{
	Use:          "compass",
	Short:        "Kubernetes & Helm",
	Long:         "Layer variables from templated files and explicit values.",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		k8s = kube.NewClient(kubeConfig)
		funcs := core.RenderWith(k8s)
		outValues = util.NewValues(inValues) // explicit cli inputs

		// additional template files
		for _, i := range templates {
			if err = outValues.FromTemplate(i, funcs); err != nil {
				return fmt.Errorf("couldn't attach import %s: %v", i, err)
			}
		}

		return nil
	},
}

var outputCmd = &cobra.Command{
	Use:     "output",
	Aliases: []string{"out"},
	Short:   "Output the generated values",
	Long:    "Print the result of layering input values as a JSON object.",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if len(outValues) == 0 {
			return fmt.Errorf("no values supplied")
		}

		if toEnv {
			outValues.ToEnv("")
		} else {
			valOut, err := json.Marshal(outValues)
			if err != nil {
				return fmt.Errorf("couldn't marshal values: %v", err)
			}
			fmt.Println(string(valOut))
		}
		return nil
	},
}

var runCmd = &cobra.Command{
	Use:          "run",
	Short:        "Run the given workflow",
	Long:         "Run the given workflow, installing resources that do not exist and upgrading those that do.",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		spec := args[0]
		ctx := context.Background()
		workflow := schema.NewWorkflow()

		// populate workflow with stages
		var data []byte
		if data, err = util.Render(spec, outValues, core.RenderWith(k8s)); err != nil {
			return err
		}
		if err = yaml.Unmarshal([]byte(data), &workflow); err != nil {
			return err
		}
		workflow.Values.Append(outValues)

		// do builds and fetch tags
		util.Combine(workflow.Build, builds)
		util.Combine(workflow.Tag, tags)
		shas := make(map[string]string, len(builds)+len(tags))
		for k, v := range workflow.Build {
			shas[k], err = docker.BuildAndPush(ctx, buildCtx, v)
			if err != nil {
				return err
			}
		}
		for k, v := range workflow.Tag {
			shas[k], err = docker.GetImageDigest(v)
			if err != nil {
				return err
			}
		}

		// we want those digests before we
		// template the main workflow
		workflow.Values.AppendStr(shas)

		tiller, err := helm.NewClient(k8s, helmConfig, tillerName, tillerPort)
		if err != nil {
			return err
		}
		defer tiller.Close()

		if err = core.Connect(workflow, k8s, tiller, workflow.Values); err != nil {
			return err
		}

		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			os.Exit(1)
		}()

		if err = core.Lint(workflow, workflow.Values); err != nil {
			return err
		}

		force := force
		if len(workflow.Stages) == 0 {
			return fmt.Errorf("nothing to run")
		}

		// reverse workflow
		if destroy {
			core.Backward(workflow.Stages, workflow.Values, force)
			return nil
		}

		// stop at desired key
		if until != "" {
			core.Until(workflow.Stages, workflow.Values, force, until)
			return nil
		}

		// run full workflow
		return core.Forward(workflow.Stages, workflow.Values, force)
	},
}

var kubeCmd = &cobra.Command{
	Use:     "kube",
	Aliases: []string{"kubernetes"},
	Short:   "Template and deploy given Kubernetes spec",
	Long:    "Install or upgrade Kubernetes objects based on the supplied specification.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spec := args[0]
		out, err := util.Render(spec, outValues, core.RenderWith(k8s))
		if err != nil {
			return err
		}

		man := kube.Manifest{
			Timeout:   300,
			Object:    out,
			Namespace: namespace,
			K8s:       k8s,
		}

		if err = man.InstallOrUpgrade(); err != nil {
			return err
		}

		log.Info("Deployed successfully")
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringToStringVar(&builds, "build", nil, "build specified image in context")
	rootCmd.PersistentFlags().StringVar(&kubeConfig, "kube-config", "", "kubernetes config")
	rootCmd.PersistentFlags().StringToStringVar(&tags, "tag", nil, "get digest of image")
	rootCmd.PersistentFlags().StringArrayVarP(&templates, "template", "t", nil, "file with key:value mappings")
	rootCmd.PersistentFlags().StringToStringVar(&inValues, "value", nil, "explicit key=value pairs")

	runCmd.Flags().StringVarP(&buildCtx, "context", "c", ".", "context for building and packaging")
	runCmd.Flags().BoolVarP(&destroy, "destroy", "d", false, "purge all stages, top-down")
	runCmd.Flags().BoolVarP(&force, "force", "f", false, "force install / upgrade / delete")
	runCmd.Flags().StringVar(&helmConfig, "helm-config", "", "helm config")
	runCmd.Flags().StringVarP(&tillerName, "tillerName", "n", "kube-system", "namespace to search for Tiller")
	runCmd.Flags().StringVarP(&tillerPort, "tillerPort", "p", "44134", "port to connect on Tiller")
	runCmd.Flags().StringVarP(&until, "until", "u", "", "only deploy stage and dependencies")
	rootCmd.AddCommand(runCmd)

	kubeCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace to deploy")
	rootCmd.AddCommand(kubeCmd)

	outputCmd.Flags().BoolVarP(&toEnv, "to-env", "e", false, "output the finalized values into environment variables")
	rootCmd.AddCommand(outputCmd)

	versionCmd.Flags().BoolVar(&shortVersion, "short", false, "only output version")
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

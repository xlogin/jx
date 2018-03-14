package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jenkins-x/jx/pkg/jx/cmd/log"
	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	cmdutil "github.com/jenkins-x/jx/pkg/jx/cmd/util"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1"
)

const (
 optionZones = "zones"

)
// CreateClusterAWSOptions contains the CLI flags
type CreateClusterAWSOptions struct {
	CreateClusterOptions

	Flags CreateClusterAWSFlags
}

type CreateClusterAWSFlags struct {
	NodeCount       string
	KubeVersion     string
	Zones     string
	UseRBAC bool
}

var (
	createClusterAWSLong = templates.LongDesc(`
		This command creates a new kubernetes cluster on Amazon Web Service (AWS) using kops, installing required local dependencies and provisions the
		Jenkins X platform

		AWS manages your hosted Kubernetes environment via kops, making it quick and easy to deploy and
		manage containerized applications without container orchestration expertise. It also eliminates the burden of
		ongoing operations and maintenance by provisioning, upgrading, and scaling resources on demand, without taking
		your applications offline.

`)

	createClusterAWSExample = templates.Examples(`

		jx create cluster aws

`)
)

// NewCmdCreateClusterAWS creates the command
func NewCmdCreateClusterAWS(f cmdutil.Factory, out io.Writer, errOut io.Writer) *cobra.Command {
	options := CreateClusterAWSOptions{
		CreateClusterOptions: createCreateClusterOptions(f, out, errOut, AKS),
	}
	cmd := &cobra.Command{
		Use:     "aws",
		Short:   "Create a new kubernetes cluster on AWS with kops",
		Long:    createClusterAWSLong,
		Example: createClusterAWSExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			cmdutil.CheckErr(err)
		},
	}

	options.addCreateClusterFlags(cmd)

	cmd.Flags().BoolVarP(&options.Flags.UseRBAC, "rbac", "r", true, "whether to enable RBAC on the Kubernetes cluster")
	cmd.Flags().StringVarP(&options.Flags.NodeCount, optionNodes, "o", "", "node count")
	cmd.Flags().StringVarP(&options.Flags.KubeVersion, optionKubernetesVersion, "v", "", "kubernetes version")
	cmd.Flags().StringVarP(&options.Flags.Zones, optionZones, "z", "", "Availability zones. Defaults to $AWS_AVAILABILITY_ZONES")
	return cmd
}

// Run runs the command
func (o *CreateClusterAWSOptions) Run() error {
	var deps []string
	d := binaryShouldBeInstalled("kops")
	if d != "" {
		deps = append(deps, d)
	}
	err := o.installMissingDependencies(deps)
	if err != nil {
		log.Errorf("%v\nPlease fix the error or install manually then try again", err)
		os.Exit(-1)
	}

	flags := &o.Flags

	nodeCount := flags.NodeCount
	if nodeCount == "" {
		prompt := &survey.Input{
			Message: "nodes",
			Default: "3",
			Help:    "number of nodes",
		}
		survey.AskOne(prompt, &nodeCount, nil)
	}

	/*
	kubeVersion := o.Flags.KubeVersion
	if kubeVersion == "" {
		prompt := &survey.Input{
			Message: "Kubernetes version",
			Default: kubeVersion,
			Help:    "The release version of kubernetes to install in the cluster",
		}
		survey.AskOne(prompt, &kubeVersion, nil)
	}
	*/

	zones := flags.Zones
	if zones == "" {
		zones = os.Getenv("AWS_AVAILABILITY_ZONES")
		if zones == "" {
			o.warnf("No AWS_AVAILABILITY_ZONES environment variable is defined or %s option!\n", optionZones)

			prompt := &survey.Input{
				Message: "Availability zones",
				Default: "",
				Help:    "The AWS Availability Zones to use for the Kubernetes cluster",
			}
			err = survey.AskOne(prompt, &zones, survey.Required)
			if err != nil {
			  return err
			}
		}
	}
	if zones == "" {
		return fmt.Errorf("No Availility zones provided!")
	}



	args := []string{}
	if flags.NodeCount != "" {
		args = append(args, "--node-count", flags.NodeCount)
	}
	if flags.KubeVersion != "" {
		args = append(args, "--kubernetes-version", flags.KubeVersion)
	}
	auth := "RBAC"
	if !flags.UseRBAC {
		auth = "AlwaysAllow"
	}
	args = append(args, "--authorization", auth, "--zones", zones, "--yes")

	// TODO allow add custom args?

	err = o.runCommand("kops", args...)
	if err != nil {
	  return err
	}

	time.Sleep(30 * time.Second)

	err = o.waitForClusterToComeUp()
	if err != nil {
	  return fmt.Errorf("Failed to wait for Kubernetes cluster to start: %s\n", err)
	}

	return o.initAndInstall(AWS)
}

func (o *CreateClusterAWSOptions) waitForClusterToComeUp() error {
	f := func() error {
		return o.runCommand("kubectl", "get", "node")
	}
	return o.retry(200, time.Second + 20, f)
}

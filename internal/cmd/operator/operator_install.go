package operator

import (
	"context"
	"errors"
	"fmt"
	"github.com/jetstack/cert-manager/cmd/ctl/pkg/factory"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/util/wait"
	"log"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/jetstack/cert-manager/cmd/ctl/pkg/check/api"
	cmcmdutil "github.com/jetstack/cert-manager/cmd/util"
	"github.com/milvus-io/milvusctl/pkg"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectlcreate "k8s.io/kubectl/pkg/cmd/create"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"time"
)

type Options *api.Options
type operatorInstallArgs struct {
	//crFilename is the path to the input milvusOperator CR.
	crFilename string

	//kubeConfigPath is the path to kube config file.
	kubeConfigPath string

	//kubectl create options
	createOptions kubectlcreate.CreateOptions
}

func NewOperatorInstallCmd(cfg *action.Configuration, f cmdutil.Factory, ioStreams genericclioptions.IOStreams, client *client.Client) *cobra.Command {
	o := kubectlcreate.NewCreateOptions(ioStreams)
	co := api.NewOptions(ioStreams)
	co.Wait = 3 * time.Minute
	co.Interval = 5 * time.Second
	co.Verbose = false
	o.FilenameOptions.Filenames = append(o.FilenameOptions.Filenames, "https://raw.githubusercontent.com/milvus-io/milvus-operator/main/deploy/manifests/deployment.yaml")
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install the milvus operator controller in the cluster",
		Long:  "The install subcommand installs the milvus operator controller in the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if cmdutil.IsFilenameSliceEmpty(o.FilenameOptions.Filenames, o.FilenameOptions.Kustomize) {
				ioStreams.ErrOut.Write([]byte("Error: must specify one of -f and -k\\n\\n"))
			}
			settings := cli.New()
			options := &pkg.InstallOptions{
				Settings:  settings,
				Cfg:       cfg,
				Client:    action.NewInstall(cfg),
				ValueOpts: &values.Options{},
				ChartName: "jetstack/cert-manager",
				DryRun:    false,
			}
			options.Client.Namespace = "cert-manager"
			options.Client.ReleaseName = "cert-manager"
			options.Client.Wait = true
			options.Client.GenerateName = false
			options.Client.CreateNamespace = true
			options.Client.DryRun = false
			if _, err := options.RunInstall(context.TODO()); err != nil {
				ioStreams.ErrOut.Write([]byte("Error: cert-manager install failed"))
			}
			if err := co.Complete(); err != nil {
				fmt.Println(err)
			}
			log.Printf("Installing the cert manager component，please wating----------------")
			run(context.TODO(), Options(co))
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.ValidateArgs(cmd, args))
			cmdutil.CheckErr(o.RunCreate(f, cmd))
			mp := make(map[string]string)
			mp["deploy"] = o.FilenameOptions.Filenames[0]
			pkg.CreateMilvusOperatorSecert(context.TODO(), mp, *client)
		},
	}
	co.Factory = factory.New(context.TODO(), installCmd)
	o.RecordFlags.AddFlags(installCmd)
	usage := "to use to create the resouce"
	cmdutil.AddFilenameOptionFlags(installCmd, &o.FilenameOptions, usage)
	cmdutil.AddValidateFlags(installCmd)
	o.PrintFlags.AddFlags(installCmd)
	cmdutil.AddApplyAnnotationFlags(installCmd)
	cmdutil.AddDryRunFlag(installCmd)
	//cmdutil.AddFieldManagerFlagVar(installCmd,)
	return installCmd
}

// Run executes check api command
func run(ctx context.Context, o Options) {
	if !o.Verbose {
		log.SetFlags(0) // Disable prefixing logs with timestamps.
	}
	log.SetOutput(o.ErrOut) // Log all intermediate errors to stderr

	pollContext, cancel := context.WithTimeout(ctx, o.Wait)
	defer cancel()

	pollErr := wait.PollImmediateUntil(o.Interval, func() (done bool, err error) {
		if err := o.APIChecker.Check(ctx); err != nil {
			if !o.Verbose && errors.Unwrap(err) != nil {
				err = errors.Unwrap(err)
			}

			//log.Printf("Not ready: %v", err)
			return false, nil
		}

		return true, nil
	}, pollContext.Done())

	log.SetOutput(o.Out) // Log conclusion to stdout

	if pollErr != nil {
		if errors.Is(pollContext.Err(), context.DeadlineExceeded) && o.Wait > 0 {
			log.Printf("Timed out after %s", o.Wait)
		}

		cmcmdutil.SetExitCode(pollContext.Err())

		runtime.Goexit() // Do soft exit (handle all defers, that should set correct exit code)
	}

	log.Printf("The cert-manager API is ready")
}

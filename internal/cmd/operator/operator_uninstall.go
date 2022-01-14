package operator

import (
	"context"
	"github.com/milvus-io/milvusctl/pkg"
	"github.com/opentracing/opentracing-go/log"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectldelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func NewOperatorUninstallCmd(cfg *action.Configuration,f cmdutil.Factory, ioStreams genericclioptions.IOStreams,client *client.Client) *cobra.Command {
	var deleteCertManager bool

	deletflags := kubectldelete.NewDeleteFlags("containing the operator to delete.")
	deleteCmd := &cobra.Command{
		Use: "uninstall",
		Short: "Uninstall the milvus operator controller in the cluster",
		Long: "The uninstall subcommand uninstalls the milvus operator controller in the cluster",
		Run: func(cmd *cobra.Command, args []string) {
			o,err := deletflags.ToOptions(nil,ioStreams)
			 mp,err := pkg.FetchDataFromSecret(context.TODO(),*client);
			 if err != nil {
				log.Error(err)
			}
			if len(o.FilenameOptions.Filenames) ==0{
				o.FilenameOptions.Filenames = append(o.FilenameOptions.Filenames,mp["deploy"])
			}
			cmdutil.CheckErr(err)
			cmdutil.CheckErr(o.Complete(f,args,cmd))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunDelete(f))
			if deleteCertManager == true {
				options := &pkg.UnInstallOptions{
					Cfg: cfg,
					Client: action.NewUninstall(cfg),
				}

				options.RunUninstall(context.TODO())
			}
			pkg.DeleteMilvusOperatorSecert(context.TODO(),*client)
		},
	}
	deletflags.AddFlags(deleteCmd)
	cmdutil.AddDryRunFlag(deleteCmd)
	deleteCmd.Flags().BoolVar(&deleteCertManager,"delete-cert-manager",false,"delete the cert-manager if it installed")
	return deleteCmd
}

// NewDeleteCommandFlags provides default flags and values for use with the "delete" command
func NewDeleteCommandFlags(usage string) *kubectldelete.DeleteFlags {
	cascadingStrategy := "background"
	gracePeriod := -1

	// setup command defaults
	all := false
	allNamespaces := false
	force := false
	ignoreNotFound := false
	now := false
	output := ""
	labelSelector := ""
	fieldSelector := ""
	timeout := time.Duration(0)
	wait := true
	raw := ""

	filenames := []string{}
	recursive := false
	kustomize := ""

	return &kubectldelete.DeleteFlags{
		// Not using helpers.go since it provides function to add '-k' for FileNameOptions, but not FileNameFlags
		FileNameFlags: &genericclioptions.FileNameFlags{Usage: usage, Filenames: &filenames, Kustomize: &kustomize, Recursive: &recursive},
		LabelSelector: &labelSelector,
		FieldSelector: &fieldSelector,

		CascadingStrategy: &cascadingStrategy,
		GracePeriod:       &gracePeriod,

		All:            &all,
		AllNamespaces:  &allNamespaces,
		Force:          &force,
		IgnoreNotFound: &ignoreNotFound,
		Now:            &now,
		Timeout:        &timeout,
		Wait:           &wait,
		Output:         &output,
		Raw:            &raw,
	}
}
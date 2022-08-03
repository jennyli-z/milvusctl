package operator

import (
	"fmt"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// "helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectlapply "k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	applyLong = templates.LongDesc(i18n.T(`
		Apply a configuration to a resource by file name or stdin.
		The resource name must be specified. This resource will be created if it doesn't exist yet.
		To use 'apply', always create the resource initially with either 'apply' or 'create --save-config'.
		JSON and YAML formats are accepted.
		Alpha Disclaimer: the --prune functionality is not yet complete. Do not use unless you are aware of what the current state is. See https://issues.k8s.io/34274.`))

	applyExample = templates.Examples(i18n.T(`
		# Apply the configuration in pod.json to a pod
		kubectl apply -f ./pod.json
		# Apply resources from a directory containing kustomization.yaml - e.g. dir/kustomization.yaml
		kubectl apply -k dir/
		# Apply the JSON passed into stdin to a pod
		cat pod.json | kubectl apply -f -
		# Note: --prune is still in Alpha
		# Apply the configuration in manifest.yaml that matches label app=nginx and delete all other resources that are not in the file and match label app=nginx
		kubectl apply --prune -f manifest.yaml -l app=nginx
		# Apply the configuration in manifest.yaml and delete all the other config maps that are not in the file
		kubectl apply --prune -f manifest.yaml --all --prune-whitelist=core/v1/ConfigMap`))

	warningNoLastAppliedConfigAnnotation = "Warning: resource %[1]s is missing the %[2]s annotation which is required by %[3]s apply. %[3]s apply should only be used on resources created declaratively by either %[3]s create --save-config or %[3]s apply. The missing annotation will be patched automatically.\n"
	warningChangesOnDeletingResource     = "Warning: Detected changes to resource %[1]s which is currently being deleted.\n"
)

type OperatorUpgradeOptions struct {
	Version      string
	ApplyOptions *kubectlapply.ApplyOptions
}

func NewOperatorUpgradeOptions(ioStreams genericclioptions.IOStreams) *OperatorUpgradeOptions {
	return &OperatorUpgradeOptions{
		Version:      "",
		ApplyOptions: kubectlapply.NewApplyOptions(ioStreams),
	}
}

func NewOperatorUpgradeCmd(f cmdutil.Factory, ioStreams genericclioptions.IOStreams, client *client.Client) *cobra.Command {
	o := NewOperatorUpgradeOptions(ioStreams)

	// o.cmdBaseName = "milvusctl"

	cmd := &cobra.Command{
		Use:                   "upgrade (-v version)",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Apply a configuration to a resource by file name or stdin"),
		Long:                  applyLong,
		Example:               applyExample,
		Run: func(cmd *cobra.Command, args []string) {
			// o.DeleteFlags.FileNameFlags.Filenames = &[]string{"https://raw.githubusercontent.com/milvus-io/milvus-operator/main/deploy/manifests/deployment.yaml"}
			// fmt.Println(*o.DeleteFlags.FileNameFlags.Filenames)
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(validateArgs(cmd, args))
			cmdutil.CheckErr(validatePruneAll(o.ApplyOptions.Prune, o.ApplyOptions.All, o.ApplyOptions.Selector))
			cmdutil.CheckErr(o.ApplyOptions.Run())
		},
	}

	// bind flag structs
	o.ApplyOptions.DeleteFlags.AddFlags(cmd)
	o.ApplyOptions.RecordFlags.AddFlags(cmd)
	o.ApplyOptions.PrintFlags.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.Version, "version", "v", o.Version, "The operator version")
	cmd.MarkPersistentFlagRequired("version")

	cmd.Flags().BoolVar(&o.ApplyOptions.Overwrite, "overwrite", o.ApplyOptions.Overwrite, "Automatically resolve conflicts between the modified and live configuration by using values from the modified configuration")
	cmd.Flags().BoolVar(&o.ApplyOptions.Prune, "prune", o.ApplyOptions.Prune, "Automatically delete resource objects, including the uninitialized ones, that do not appear in the configs and are created by either apply or create --save-config. Should be used with either -l or --all.")
	cmdutil.AddValidateFlags(cmd)
	cmd.Flags().StringVarP(&o.ApplyOptions.Selector, "selector", "l", o.ApplyOptions.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVar(&o.ApplyOptions.All, "all", o.ApplyOptions.All, "Select all resources in the namespace of the specified resource types.")
	cmd.Flags().StringArrayVar(&o.ApplyOptions.PruneWhitelist, "prune-whitelist", o.ApplyOptions.PruneWhitelist, "Overwrite the default whitelist with <group/version/kind> for --prune")
	cmd.Flags().BoolVar(&o.ApplyOptions.OpenAPIPatch, "openapi-patch", o.ApplyOptions.OpenAPIPatch, "If true, use openapi to calculate diff when the openapi presents and the resource can be found in the openapi spec. Otherwise, fall back to use baked-in types.")
	cmdutil.AddDryRunFlag(cmd)
	cmdutil.AddServerSideApplyFlags(cmd)
	cmdutil.AddFieldManagerFlagVar(cmd, &o.ApplyOptions.FieldManager, "kubectl-client-side-apply")

	// apply subcommands
	// cmd.AddCommand(NewCmdApplyViewLastApplied(f, ioStreams))
	// cmd.AddCommand(NewCmdApplySetLastApplied(f, ioStreams))
	// cmd.AddCommand(NewCmdApplyEditLastApplied(f, ioStreams))

	return cmd
}

func (o *OperatorUpgradeOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	fileName := fmt.Sprintf("https://raw.githubusercontent.com/milvus-io/milvus-operator/v%s/deploy/manifests/deployment.yaml", o.Version)
	o.ApplyOptions.DeleteFlags.FileNameFlags.Filenames = &[]string{fileName}

	err := o.ApplyOptions.Complete(f, cmd)

	return err
}

func validateArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmdutil.UsageErrorf(cmd, "Unexpected args: %v", args)
	}
	return nil
}

func validatePruneAll(prune, all bool, selector string) error {
	if all && len(selector) > 0 {
		return fmt.Errorf("cannot set --all and --selector at the same time")
	}
	if prune && !all && selector == "" {
		return fmt.Errorf("all resources selected for prune without explicitly passing --all. To prune all resources, pass the --all flag. If you did not mean to prune all resources, specify a label selector")
	}
	return nil
}

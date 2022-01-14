package operator

import (
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewOperatorCmd(cfg *action.Configuration,f cmdutil.Factory, ioStreams genericclioptions.IOStreams,client *client.Client) *cobra.Command {
	operatorCmd := &cobra.Command{
		Use: "operator",
		Short: "command related to The Milvus operator",
		Run: runHelp,
	}
	operatorCmd.AddCommand(NewOperatorInstallCmd(cfg,f,ioStreams,client))
	operatorCmd.AddCommand(NewOperatorUninstallCmd(cfg,f,ioStreams,client))
	return operatorCmd
}
func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}
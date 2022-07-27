package exec

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectlexec "k8s.io/kubectl/pkg/cmd/exec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"time"
)

var (
	execExample = templates.Examples(i18n.T(`
		# Get output from running the 'date' command from pod mypod, using the first container by default
		milvusctl exec mypod -- date
		# Get output from running the 'date' command in ruby-container from pod mypod
		milvusctl exec mypod -c ruby-container -- date
		# Switch to raw terminal mode; sends stdin to 'bash' in ruby-container from pod mypod
		# and sends stdout/stderr from 'bash' back to the client
		milvusctl exec mypod -c ruby-container -i -t -- bash -il
		# List contents of /usr from the first container of pod mypod and sort by modification time
		# If the command you want to execute in the pod has any flags in common (e.g. -i),
		# you must use two dashes (--) to separate your command's flags/arguments
		# Also note, do not surround your command and its flags/arguments with quotes
		# unless that is how you would execute it normally (i.e., do ls -t /usr, not "ls -t /usr")
		milvusctl exec mypod -i -t -- ls -t /usr
		# Get output from running 'date' command from the first pod of the deployment mydeployment, using the first container by default
		milvusctl exec deploy/mydeployment -- date
		# Get output from running 'date' command from the first pod of the service myservice, using the first container by default
		milvusctl exec svc/myservice -- date
		`))
)

const (
	defaultPodExecTimeout = 60 * time.Second
)

func NewMilvusExecCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	options := &kubectlexec.ExecOptions{
		StreamOptions: kubectlexec.StreamOptions{
			IOStreams: streams,
		},

		Executor: &kubectlexec.DefaultRemoteExecutor{},
	}
	cmd := &cobra.Command{
		Use:                   "exec (POD | TYPE/NAME) [-c CONTAINER] [flags] -- COMMAND [args...]",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Execute a command in a container"),
		Long:                  i18n.T("Execute a command in a container."),
		Example:               execExample,
		ValidArgsFunction:     util.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			argsLenAtDash := cmd.ArgsLenAtDash()
			cmdutil.CheckErr(options.Complete(f, cmd, args, argsLenAtDash))
			cmdutil.CheckErr(options.Validate())
			cmdutil.CheckErr(options.Run())
		},
	}
	cmdutil.AddPodRunningTimeoutFlag(cmd, defaultPodExecTimeout)
	cmdutil.AddJsonFilenameFlag(cmd.Flags(), &options.FilenameOptions.Filenames, "to use to exec into the resource")
	// TODO support UID
	cmdutil.AddContainerVarFlags(cmd, &options.ContainerName, options.ContainerName)
	cmd.Flags().BoolVarP(&options.Stdin, "stdin", "i", options.Stdin, "Pass stdin to the container")
	cmd.Flags().BoolVarP(&options.TTY, "tty", "t", options.TTY, "Stdin is a TTY")
	cmd.Flags().BoolVarP(&options.Quiet, "quiet", "q", options.Quiet, "Only print output from the remote session")
	return cmd
}

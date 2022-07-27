/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"github.com/milvus-io/milvusctl/internal/cmd/create"
	"github.com/milvus-io/milvusctl/internal/cmd/delete"
	"github.com/milvus-io/milvusctl/internal/cmd/describe"
	"github.com/milvus-io/milvusctl/internal/cmd/logs"
	"github.com/milvus-io/milvusctl/internal/cmd/operator"
	"github.com/milvus-io/milvusctl/internal/cmd/portforward"
	"github.com/milvus-io/milvusctl/internal/cmd/update"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/plugin"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"os"
	"os/exec"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"syscall"
)

type MilvusctlOptions struct {
	PluginHandler PluginHandler
	Arguments     []string
	ConfigFlags   *genericclioptions.ConfigFlags
	genericclioptions.IOStreams
}

// rootCmd represents the base command when called without any subcommands
func NewMilvusCmd(cfg *action.Configuration, client *client.Client) *cobra.Command {
	o := MilvusctlOptions{
		PluginHandler: NewDefaultPluginHandler(plugin.ValidPluginFilenamePrefixes),
		Arguments:     os.Args,
		ConfigFlags:   defaultConfigFlags,
		IOStreams:     genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	var milvusCmd = &cobra.Command{
		Use:   "milvusctl",
		Short: "milvus application controls interface",
		Long: `milvus configuration command line utility for service operators to deploy and diagnose their milvus application

Find more information at:
	https://github.com/milvus-io/milvusctl
		`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		Run: runHelp,
	}

	flags := milvusCmd.PersistentFlags()
	kubeConfigFlags := o.ConfigFlags
	if kubeConfigFlags == nil {
		kubeConfigFlags = defaultConfigFlags
	}
	kubeConfigFlags.AddFlags(flags)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(flags)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	milvusCmd.AddCommand(operator.NewOperatorCmd(cfg, f, o.IOStreams, client))
	milvusCmd.AddCommand(create.NewMilvusCreateCmd(f, o.IOStreams, client))
	milvusCmd.AddCommand(portforward.NewPortForwardCmd(f, o.IOStreams))
	milvusCmd.AddCommand(delete.NewMilvusDeleteCmd(f, o.IOStreams, client))
	milvusCmd.AddCommand(update.NewMilvusUpdateCmd(f, o.IOStreams, client))
	milvusCmd.AddCommand(logs.NewMilvusLogsCmd(f, o.IOStreams))
	milvusCmd.AddCommand(describe.NewMilvusDescribeCmd(f, o.IOStreams, client))
	return milvusCmd
}

var defaultConfigFlags = genericclioptions.NewConfigFlags(true)

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}

// PluginHandler is capable of parsing command line arguments
// and performing executable filename lookups to search
// for valid plugin files, and execute found plugins.
type PluginHandler interface {
	// exists at the given filename, or a boolean false.
	// Lookup will iterate over a list of given prefixes
	// in order to recognize valid plugin filenames.
	// The first filepath to match a prefix is returned.
	Lookup(filename string) (string, bool)
	// Execute receives an executable's filepath, a slice
	// of arguments, and a slice of environment variables
	// to relay to the executable.
	Execute(executablePath string, cmdArgs, environment []string) error
}

// DefaultPluginHandler implements PluginHandler
type DefaultPluginHandler struct {
	ValidPrefixes []string
}

// Lookup implements PluginHandler
func (h *DefaultPluginHandler) Lookup(filename string) (string, bool) {
	for _, prefix := range h.ValidPrefixes {
		path, err := exec.LookPath(fmt.Sprintf("%s-%s", prefix, filename))
		if err != nil || len(path) == 0 {
			continue
		}
		return path, true
	}

	return "", false
}

// Execute implements PluginHandler
func (h *DefaultPluginHandler) Execute(executablePath string, cmdArgs, environment []string) error {

	// Windows does not support exec syscall.
	if runtime.GOOS == "windows" {
		cmd := exec.Command(executablePath, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = environment
		err := cmd.Run()
		if err == nil {
			os.Exit(0)
		}
		return err
	}

	// invoke cmd binary relaying the environment and args given
	// append executablePath to cmdArgs, as execve will make first argument the "binary name".
	return syscall.Exec(executablePath, append([]string{executablePath}, cmdArgs...), environment)
}

// NewDefaultPluginHandler instantiates the DefaultPluginHandler with a list of
// given filename prefixes used to identify valid plugin filenames.
func NewDefaultPluginHandler(validPrefixes []string) *DefaultPluginHandler {
	return &DefaultPluginHandler{
		ValidPrefixes: validPrefixes,
	}
}

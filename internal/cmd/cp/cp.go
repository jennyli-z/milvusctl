package cp

import (
	// "archive/tar"
	// "bytes"
	// "errors"
	"fmt"
	// "io"
	"io/ioutil"
	// "os"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	// "k8s.io/client-go/kubernetes"
	// restclient "k8s.io/client-go/rest"
	kubectlcp "k8s.io/kubectl/pkg/cmd/cp"
	// "k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	cpExample = templates.Examples(i18n.T(`
		# !!!Important Note!!!
		# Requires that the 'tar' binary is present in your container
		# image.  If 'tar' is not present, 'milvusctl cp' will fail.
		#
		# For advanced use cases, such as symlinks, wildcard expansion or
		# file mode preservation, consider using 'milvusctl exec'.
		# Copy /tmp/foo local file to /tmp/bar in a remote pod in namespace <some-namespace>
		tar cf - /tmp/foo | milvusctl exec -i -n <some-namespace> <some-pod> -- tar xf - -C /tmp/bar
		# Copy /tmp/foo from a remote pod to /tmp/bar locally
		milvusctl exec -n <some-namespace> <some-pod> -- tar cf - /tmp/foo | tar xf - -C /tmp/bar
		# Copy /tmp/foo_dir local directory to /tmp/bar_dir in a remote pod in the default namespace
		milvusctl cp /tmp/foo_dir <some-pod>:/tmp/bar_dir
		# Copy /tmp/foo local file to /tmp/bar in a remote pod in a specific container
		milvusctl cp /tmp/foo <some-pod>:/tmp/bar -c <specific-container>
		# Copy /tmp/foo local file to /tmp/bar in a remote pod in namespace <some-namespace>
		milvusctl cp /tmp/foo <some-namespace>/<some-pod>:/tmp/bar
		# Copy /tmp/foo from a remote pod to /tmp/bar locally
		milvusctl cp <some-namespace>/<some-pod>:/tmp/foo /tmp/bar`))
)

func NewMilvusCpCmd(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := kubectlcp.NewCopyOptions(ioStreams)

	cmd := &cobra.Command{
		Use:                   "cp <file-spec-src> <file-spec-dest>",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Copy files and directories to and from containers"),
		Long:                  i18n.T("Copy files and directories to and from containers."),
		Example:               cpExample,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var comps []string
			if len(args) == 0 {
				if strings.IndexAny(toComplete, "/.~") == 0 {
					// Looks like a path, do nothing
				} else if strings.Index(toComplete, ":") != -1 {
					// TODO: complete remote files in the pod
				} else if idx := strings.Index(toComplete, "/"); idx > 0 {
					// complete <namespace>/<pod>
					namespace := toComplete[:idx]
					template := "{{ range .items }}{{ .metadata.namespace }}/{{ .metadata.name }}: {{ end }}"
					comps = get.CompGetFromTemplate(&template, f, namespace, cmd, []string{"pod"}, toComplete)
				} else {
					// Complete namespaces followed by a /
					for _, ns := range get.CompGetResource(f, cmd, "namespace", toComplete) {
						comps = append(comps, fmt.Sprintf("%s/", ns))
					}
					// Complete pod names followed by a :
					for _, pod := range get.CompGetResource(f, cmd, "pod", toComplete) {
						comps = append(comps, fmt.Sprintf("%s:", pod))
					}

					// Finally, provide file completion if we need to.
					// We only do this if:
					// 1- There are other completions found (if there are no completions,
					//    the shell will do file completion itself)
					// 2- If there is some input from the user (or else we will end up
					//    listing the entire content of the current directory which could
					//    be too many choices for the user)
					if len(comps) > 0 && len(toComplete) > 0 {
						if files, err := ioutil.ReadDir("."); err == nil {
							for _, file := range files {
								filename := file.Name()
								if strings.HasPrefix(filename, toComplete) {
									if file.IsDir() {
										filename = fmt.Sprintf("%s/", filename)
									}
									// We are completing a file prefix
									comps = append(comps, filename)
								}
							}
						}
					} else if len(toComplete) == 0 {
						// If the user didn't provide any input to complete,
						// we provide a hint that a path can also be used
						comps = append(comps, "./", "/")
					}
				}
			}
			return comps, cobra.ShellCompDirectiveNoSpace
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Validate(cmd, args))
			cmdutil.CheckErr(o.Run(args))
		},
	}
	cmdutil.AddContainerVarFlags(cmd, &o.Container, o.Container)
	cmd.Flags().BoolVarP(&o.NoPreserve, "no-preserve", "", false, "The copied file/directory's ownership and permissions will not be preserved in the container")

	return cmd
}

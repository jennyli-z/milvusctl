package logs

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	kubectllogs "k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"path/filepath"
)

const (
	logsUsageStr = "logs [-f] [-p] (POD | TYPE/NAME) [-c CONTAINER]"
)

var (
	logsLong = templates.LongDesc(i18n.T(`
		Print the logs for a container in a pod or specified resource 
		If the pod has only one container, the container name is optional.
		Or export the logs for Milvus and its' dependences.`))

	logsExample = templates.Examples(i18n.T(`
		# Save all Milvus component and its' dependences logs
		milvusctl logs milvus/milvus-release --all
		# Save the specify milvus componenet and dependences logs
		milvusctl logs milvus/milvus-release --component=datanode --etcd
		# Return snapshot logs from pod nginx with only one container
		milvusctl logs nginx
		# Return snapshot logs from pod nginx with multi containers
		milvusctl logs nginx --all-containers=true
		# Return snapshot logs from all containers in pods defined by label app=nginx
		milvusctl logs -l app=nginx --all-containers=true
		# Return snapshot of previous terminated ruby container logs from pod web-1
		milvusctl logs -p -c ruby web-1
		# Begin streaming the logs of the ruby container in pod web-1
		milvusctl logs -f -c ruby web-1
		# Begin streaming the logs from all containers in pods defined by label app=nginx
		milvusctl logs -f -l app=nginx --all-containers=true
		# Display only the most recent 20 lines of output in pod nginx
		milvusctl logs --tail=20 nginx
		# Show all logs from pod nginx written in the last hour
		milvusctl logs --since=1h nginx
		# Show logs from a kubelet with an expired serving certificate
		milvusctl logs --insecure-skip-tls-verify-backend nginx
		# Return snapshot logs from first container of a job named hello
		milvusctl logs job/hello
		# Return snapshot logs from container nginx-1 of a deployment named nginx
		milvusctl logs deployment/nginx -c nginx-1`))

	selectorTail    int64 = 10
	logsUsageErrStr       = fmt.Sprintf("expected '%s'.\nPOD or TYPE/NAME is a required argument for the logs command", logsUsageStr)
)

const (
	defaultPodLogsTimeout = 20 * time.Second
)

type MilvusLogsOptions struct {
	FilePath         string
	Namespace        string
	InstanceName     string
	SaveAll          bool
	MilvusComponenet string
	Etcd             bool
	Minio            bool
	Pulsar           bool
	Kafka            bool
	LogsOptions      *kubectllogs.LogsOptions
}

func NewMilvusLogsOptions(streams genericclioptions.IOStreams, allContainers bool) *MilvusLogsOptions {
	return &MilvusLogsOptions{
		FilePath:         "./milvus-logs",
		Namespace:        "default",
		MilvusComponenet: "all",
		LogsOptions:      kubectllogs.NewLogsOptions(streams, allContainers),
	}
}

func NewMilvusLogsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMilvusLogsOptions(streams, false)

	logsCmd := &cobra.Command{
		Use:                   logsUsageStr,
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Print the logs for a container in a pod"),
		Long:                  logsLong,
		Example:               logsExample,
		// Args:                  cobra.MaximumNArgs(1),
		ValidArgsFunction: util.PodResourceNameAndContainerCompletionFunc(f),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 1 && args[0][:7] == "milvus/" {
				// fmt.Println("export milvus logs")
				cmdutil.CheckErr(o.Complete(f, cmd, args))
				cmdutil.CheckErr(o.Validate())
				cmdutil.CheckErr(o.RunLogs(f, cmd, args))
			} else {
				cmdutil.CheckErr(o.LogsOptions.Complete(f, cmd, args))
				cmdutil.CheckErr(o.LogsOptions.Validate())
				cmdutil.CheckErr(o.LogsOptions.RunLogs())
			}
		},
	}
	o.LogsOptions.AddFlags(logsCmd)

	logsCmd.Flags().StringVarP(&o.FilePath, "dir", "d", o.FilePath, "Specify the path where the logs saved")
	logsCmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "use type parameter to choose install namespace")
	logsCmd.Flags().BoolVar(&o.SaveAll, "all", o.SaveAll, "Specify if saved all the logs of Milvus and dependences")
	logsCmd.Flags().StringVar(&o.MilvusComponenet, "component", o.MilvusComponenet, "choose which milvus component's logs to export.")
	logsCmd.Flags().BoolVar(&o.Etcd, "etcd", o.Etcd, "Specify if saved the logs of etcd")
	logsCmd.Flags().BoolVar(&o.Minio, "minio", o.Minio, "Specify if saved the logs of minio")
	logsCmd.Flags().BoolVar(&o.Pulsar, "pulsar", o.Pulsar, "Specify if saved the logs of pulsar")
	logsCmd.Flags().BoolVar(&o.Kafka, "Kafka", o.Kafka, "Specify if saved the logs of kafka")
	return logsCmd
}

func (o MilvusLogsOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.InstanceName = args[0][7:]
	dir := filepath.Join(o.FilePath, o.InstanceName)
	if !Exists(o.FilePath) {
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
		fmt.Printf("Milvus logs will store in %s \n", dir)
	} else {
		if IsDir(o.FilePath) {
			if Exists(dir) {
				return fmt.Errorf("Path %s already exists, remove it or specify a new path", dir)
			} else {
				if err = os.MkdirAll(dir, os.ModePerm); err != nil {
					return err
				}
				fmt.Printf("Milvus logs will store in %s \n", dir)
			}
		} else {
			return fmt.Errorf("The path %s is not a dir", o.FilePath)
		}
	}

	return err
}

func (o MilvusLogsOptions) Validate() error {
	if o.SaveAll && o.MilvusComponenet != "all" {
		return fmt.Errorf("Parameter ‘component’ and ‘all’ conflict, only need to specify one of them")
	}
	return nil
}

func (o MilvusLogsOptions) RunLogs(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.InstanceName = args[0][7:]
	if o.SaveAll {
		if err := o.SaveMilvusComLogs(f, cmd); err != nil {
			fmt.Println(err)
		}
		if err := o.SaveEtcdComLogs(f, cmd); err != nil {
			fmt.Println(err)
		}
		if err := o.SavePulsarComLogs(f, cmd); err != nil {
			fmt.Println(err)
		}
		if err := o.SaveKafkaComLogs(f, cmd); err != nil {
			fmt.Println(err)
		}
		if err := o.SaveMinioComLogs(f, cmd); err != nil {
			fmt.Println(err)
		}
	} else {
		if err := o.SaveMilvusComLogs(f, cmd); err != nil {
			fmt.Println(err)
		}
		if o.Etcd {
			if err := o.SaveEtcdComLogs(f, cmd); err != nil {
				fmt.Println(err)
			}
		}
		if o.Pulsar {
			if err := o.SavePulsarComLogs(f, cmd); err != nil {
				fmt.Println(err)
			}
		}
		if o.Kafka {
			if err := o.SaveKafkaComLogs(f, cmd); err != nil {
				fmt.Println(err)
			}
		}
		if o.Minio {
			if err := o.SaveMinioComLogs(f, cmd); err != nil {
				fmt.Println(err)
			}
		}
	}
	zipDir := filepath.Join(o.FilePath, o.InstanceName+".gz")
	if err := Zip(filepath.Join(o.FilePath, o.InstanceName), zipDir); err != nil {
		return err
	}

	return nil
}

func (o MilvusLogsOptions) SaveMilvusComLogs(f cmdutil.Factory, cmd *cobra.Command) error {
	labels := "app.kubernetes.io/instance=" + o.InstanceName + ", app.kubernetes.io/name=milvus"
	// fmt.Println("milvus labels: ", labels)
	var err error
	switch o.MilvusComponenet {
	case "proxy":
		labels = labels + ", app.kubernetes.io/component=proxy"
		err = o.writeLogs(f, cmd, labels, "milvus proxy")
	case "rootcoord":
		labels = labels + ", app.kubernetes.io/component=rootcoord"
		err = o.writeLogs(f, cmd, labels, "milvus rootcoord")
	case "mixcoord":
		labels = labels + ", app.kubernetes.io/component=mixcoord"
		err = o.writeLogs(f, cmd, labels, "milvus mixcoord")
	case "indexcoord":
		labels = labels + ", app.kubernetes.io/component=indexcoord"
		err = o.writeLogs(f, cmd, labels, "milvus indexcoord")
	case "querycoord":
		labels = labels + ", app.kubernetes.io/component=querycoord"
		err = o.writeLogs(f, cmd, labels, "milvus querycoord")
	case "datacoord":
		labels = labels + ", app.kubernetes.io/component=datacoord"
		err = o.writeLogs(f, cmd, labels, "milvus datacoord")
	case "indexnode":
		labels = labels + ", app.kubernetes.io/component=indexnode"
		err = o.writeLogs(f, cmd, labels, "milvus indexnode")
	case "querynode":
		labels = labels + ", app.kubernetes.io/component=querynode"
		err = o.writeLogs(f, cmd, labels, "milvus querynode")
	case "datanode":
		labels = labels + ", app.kubernetes.io/component=datanode"
		err = o.writeLogs(f, cmd, labels, "milvus datanode")
	case "all":
		err = o.writeLogs(f, cmd, labels, "milvus")
	default:
		return fmt.Errorf("Milvus component error: %s. choose one of them: proxy, rootcoord, mixcoord, indexcoord, querycoord, datacoord, indexnode, querynode, datanode, all", o.MilvusComponenet)
	}

	return err
}

func (o MilvusLogsOptions) SaveEtcdComLogs(f cmdutil.Factory, cmd *cobra.Command) error {
	labels := "app.kubernetes.io/instance=" + o.InstanceName + "-etcd, app.kubernetes.io/name=etcd"
	err := o.writeLogs(f, cmd, labels, "etcd")
	return err
}

func (o MilvusLogsOptions) SavePulsarComLogs(f cmdutil.Factory, cmd *cobra.Command) error {
	labels := "cluster=" + o.InstanceName + "-pulsar, app=pulsar"
	err := o.writeLogs(f, cmd, labels, "pulsar")
	return err
}
func (o MilvusLogsOptions) SaveKafkaComLogs(f cmdutil.Factory, cmd *cobra.Command) error {
	labels := "app.kubernetes.io/instance=" + o.InstanceName + "-kafka, app.kubernetes.io/component=kafka"
	err := o.writeLogs(f, cmd, labels, "kafka")
	return err
}
func (o MilvusLogsOptions) SaveMinioComLogs(f cmdutil.Factory, cmd *cobra.Command) error {
	labels := "release=" + o.InstanceName + "-minio, app=minio"
	err := o.writeLogs(f, cmd, labels, "minio")
	return err
}

func (o MilvusLogsOptions) writeLogs(f cmdutil.Factory, cmd *cobra.Command, labels string, com string) error {
	requests, err := o.GetObjRequest(f, cmd, labels)
	if err != nil {
		return nil
	}
	if len(requests) != 0 {
		fmt.Printf("%v pods founded for the %s component, export the log of them \n", len(requests), com)
	}
	for objRef, request := range requests {
		fileName := filepath.Join(o.FilePath, o.InstanceName, objRef.Name+".log")
		err := SavePodLogs(fileName, request)
		if err != nil {
			return err
			// fmt.Println(err)
		}
	}
	return nil
}

func (o MilvusLogsOptions) GetObjRequest(f cmdutil.Factory, cmd *cobra.Command, labels string) (map[corev1.ObjectReference]rest.ResponseWrapper, error) {
	var RESTClientGetter genericclioptions.RESTClientGetter
	RESTClientGetter = f

	builder := f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(o.Namespace).DefaultNamespace().
		SingleResourceType()
	builder.ResourceTypes("pods").LabelSelectorParam(labels)

	// for _, label := range labels {
	// 	builder.LabelSelectorParam(label)
	// }

	infos, err := builder.Do().Infos()
	if err != nil {
		return nil, err
	}

	object := infos[0].Object
	// fmt.Println(len(object.(*corev1.PodList).Items))

	getPodTimeout, err := cmdutil.GetPodRunningTimeoutFlag(cmd)
	if err != nil {
		return nil, err
	}

	requests, err := polymorphichelpers.LogsForObjectFn(RESTClientGetter, object, &corev1.PodLogOptions{}, getPodTimeout, false)
	if err != nil {
		return nil, err
	}

	return requests, nil
}

func SavePodLogs(fileName string, request rest.ResponseWrapper) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := DefaultConsumeRequest(request, file); err != nil {
		return err
	}
	return nil
}

func DefaultConsumeRequest(request rest.ResponseWrapper, f *os.File) error {
	readCloser, err := request.Stream(context.TODO())
	if err != nil {
		return err
	}
	defer readCloser.Close()

	r := bufio.NewReader(readCloser)
	for {
		bytes, err := r.ReadBytes('\n')
		if _, err := f.Write(bytes); err != nil {
			return err
		}

		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

// if the given path file/folder exists
func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

// if the given path is a folder
func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func Zip(srcDir string, zipFileName string) error {
	if Exists(zipFileName) {
		return fmt.Errorf("Unable to compress: file %s already exists, remove it first", zipFileName)
	}
	// os.RemoveAll(zip_file_name)

	// Create zip file
	zipFile, _ := os.Create(zipFileName)
	defer zipFile.Close()

	// Open the zip file
	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	filepath.Walk(srcDir, func(path string, info os.FileInfo, _ error) error {

		if path == srcDir {
			return nil
		}

		header, _ := zip.FileInfoHeader(info)
		header.Name = strings.TrimPrefix(path, srcDir+`/`)

		if info.IsDir() {
			header.Name += `/`
		} else {
			header.Method = zip.Deflate
		}

		writer, _ := archive.CreateHeader(header)
		if !info.IsDir() {
			file, _ := os.Open(path)
			defer file.Close()
			io.Copy(writer, file)
		}
		return nil
	})

	return nil
}

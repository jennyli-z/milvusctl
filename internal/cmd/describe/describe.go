package describe

import (
	"context"
	"fmt"
	"github.com/milvus-io/milvus-operator/apis/milvus.io/v1beta1"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	kubectldescribe "k8s.io/kubectl/pkg/cmd/describe"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"
	"k8s.io/kubectl/pkg/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"
	conclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var (
	describeLong = templates.LongDesc(i18n.T(`
		Show details of a specific resource or group of resources.
		Print a detailed description of the selected resources, including related resources such
		as events or controllers. You may select a single object by name, all objects of that
		type, provide a name prefix, or label selector. For example:
		    $ milvusctl describe TYPE NAME_PREFIX
		will first check for an exact match on TYPE and NAME_PREFIX. If no such resource
		exists, it will output details for every resource that has a name prefixed with NAME_PREFIX.`))

	describeExample = templates.Examples(i18n.T(`
		# Describe a node
		milvusctl describe nodes kubernetes-node-emt8.c.myproject.internal
		# Describe a pod
		milvusctl describe pods/nginx
		# Describe a pod identified by type and name in "pod.json"
		milvusctl describe -f pod.json
		# Describe all pods
		milvusctl describe pods
		# Describe pods by label name=myLabel
		milvusctl describe po -l name=myLabel
		# Describe all pods managed by the 'frontend' replication controller (rc-created pods
		# get the name of the rc as a prefix in the pod the name)
		milvusctl describe pods frontend`))
)

type MilvusDescribeOptions struct {
	Namespace       string
	InstanceName    string
	Component       string
	Dependence      string
	DescribeOptions *kubectldescribe.DescribeOptions
}

func NewLogsDescribe(parent string, f cmdutil.Factory, streams genericclioptions.IOStreams) *kubectldescribe.DescribeOptions {
	return &kubectldescribe.DescribeOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
			ChunkSize:  cmdutil.DefaultChunkSize,
		},
		CmdParent: parent,

		IOStreams: streams,
	}
}

func NewMilvusDescribeOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *MilvusDescribeOptions {
	return &MilvusDescribeOptions{
		Namespace:       "default",
		DescribeOptions: NewLogsDescribe("milvusctl", f, streams),
	}
}

func NewMilvusDescribeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams, client *client.Client) *cobra.Command {
	o := NewMilvusDescribeOptions(f, streams)

	cmd := &cobra.Command{
		Use:                   "describe (-f FILENAME | TYPE [NAME_PREFIX | -l label] | TYPE/NAME)",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Show details of a specific resource or group of resources"),
		Long:                  describeLong + "\n\n" + cmdutil.SuggestAPIResources("milvusctl"),
		Example:               describeExample,
		ValidArgsFunction:     util.ResourceTypeAndNameCompletionFunc(f),
		Run: func(cmd *cobra.Command, args []string) {
			if args[0] == "milvus" && (o.Component != "" || o.Dependence != "") {
				cmdutil.CheckErr(o.Complete(args))
				cmdutil.CheckErr(o.Run(f, client, args))
			} else {
				cmdutil.CheckErr(o.DescribeOptions.Complete(f, cmd, args))
				cmdutil.CheckErr(o.DescribeOptions.Run())
			}

		},
	}

	usage := "containing the resource to describe"
	cmdutil.AddFilenameOptionFlags(cmd, o.DescribeOptions.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.DescribeOptions.Selector, "selector", "l", o.DescribeOptions.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVarP(&o.DescribeOptions.AllNamespaces, "all-namespaces", "A", o.DescribeOptions.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.DescribeOptions.DescriberSettings.ShowEvents, "show-events", o.DescribeOptions.DescriberSettings.ShowEvents, "If true, display events related to the described object.")
	cmdutil.AddChunkSizeFlag(cmd, &o.DescribeOptions.DescriberSettings.ChunkSize)

	cmd.Flags().StringVar(&o.Component, "component", o.Component, "specify milvus componenet")
	cmd.Flags().StringVar(&o.Dependence, "dependence", o.Dependence, "specify milvus dependence")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "use type parameter to choose namespace")

	return cmd
}

func (o MilvusDescribeOptions) Complete(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("Need two parameters for describe Milvus status: milvus, instance_name")
	}
	return nil
}

func (o MilvusDescribeOptions) Run(f cmdutil.Factory, client *client.Client, args []string) error {
	var err error
	allErrs := []error{}
	o.InstanceName = args[1]
	err = o.MilvusComponent(f, *client)
	if err != nil {
		allErrs = append(allErrs, err)
	}
	err = o.MilvusDependence(f, *client)
	if err != nil {
		allErrs = append(allErrs, err)
	}
	return utilerrors.NewAggregate(allErrs)
}

func (o MilvusDescribeOptions) MilvusComponent(f cmdutil.Factory, client client.Client) error {
	var err error
	switch o.Component {
	case "proxy":
		err = o.DescribeMilvusComponent(f, client, "proxy")
	case "rootcoord":
		err = o.DescribeMilvusComponent(f, client, "rootcoord")
	case "mixcoord":
		err = o.DescribeMilvusComponent(f, client, "mixcoord")
	case "indexcoord":
		err = o.DescribeMilvusComponent(f, client, "indexcoord")
	case "querycoord":
		err = o.DescribeMilvusComponent(f, client, "querycoord")
	case "datacoord":
		err = o.DescribeMilvusComponent(f, client, "datacoord")
	case "indexnode":
		err = o.DescribeMilvusComponent(f, client, "indexnode")
	case "querynode":
		err = o.DescribeMilvusComponent(f, client, "querynode")
	case "datanode":
		err = o.DescribeMilvusComponent(f, client, "datanode")
	case "all":
		err = PrintMilvusStatus(client, o.Namespace, o.InstanceName)
	case "":
	default:
		return fmt.Errorf("component parameter error: %s. choose one of them: proxy, rootcoord, mixcoord, indexcoord, querycoord, datacoord, indexnode, querynode, datanode, all", o.Component)
	}
	return err
}

func (o MilvusDescribeOptions) MilvusDependence(f cmdutil.Factory, client client.Client) error {
	var err error
	selectorEtcd := "app.kubernetes.io/instance=" + o.InstanceName + "-etcd, app.kubernetes.io/name=etcd"
	selectorMinio := "release=" + o.InstanceName + "-minio, app=minio"
	selectorPulsar := "cluster=" + o.InstanceName + "-pulsar, app=pulsar"
	selectorKafka := "app.kubernetes.io/instance=" + o.InstanceName + "-kafka, app.kubernetes.io/component=kafka"
	switch o.Dependence {
	case "":
	case "etcd":
		err = o.DescribeMilvusDependence(f, client, "etcd", selectorEtcd)
	case "minio":
		err = o.DescribeMilvusDependence(f, client, "minio", selectorMinio)
	case "pulsar":
		err = o.DescribePulsar(f, client, selectorPulsar)
	case "kafka":
		err = o.DescribeMilvusDependence(f, client, "kafka", selectorKafka)
	case "all":
		allErrs := []error{}
		err = o.DescribeMilvusDependence(f, client, "etcd", selectorEtcd)
		if err != nil {
			allErrs = append(allErrs, err)
		}
		err = o.DescribeMilvusDependence(f, client, "minio", selectorMinio)
		if err != nil {
			allErrs = append(allErrs, err)
		}
		err = o.DescribeMilvusDependence(f, client, "kafka", selectorKafka)
		if err != nil {
			allErrs = append(allErrs, err)
		}
		err = o.DescribePulsar(f, client, selectorPulsar)
		if err != nil {
			allErrs = append(allErrs, err)
		}
		return utilerrors.NewAggregate(allErrs)
	default:
		return fmt.Errorf("dependence parameter error: %s. choose one of them: etcd, minio, pulsar, kafka, all", o.Dependence)
	}
	return err
}

func (o MilvusDescribeOptions) DescribeMilvusComponent(f cmdutil.Factory, client client.Client, component string) error {
	var err error
	allErrs := []error{}
	selector := "app.kubernetes.io/name=milvus, app.kubernetes.io/instance=" + o.InstanceName + ", app.kubernetes.io/component=" + component
	infos, err := o.GetResource(f, []string{"deployment"}, selector)

	if err != nil {
		return err
	}
	if len(infos) == 0 {
		return nil
	}

	fmt.Printf("Milvus Component Status (%s): \n", component)
	for _, info := range infos {
		err = PrintDeploymentStatus(client, info)
		if err != nil {
			allErrs = append(allErrs, err)
		}
		podSelector := "app.kubernetes.io/name=milvus, app.kubernetes.io/instance=" + o.InstanceName + ", app.kubernetes.io/component=" + component
		podInfos, err := o.GetResource(f, []string{"pod"}, podSelector)
		if err != nil {
			allErrs = append(allErrs, err)
			return utilerrors.NewAggregate(allErrs)
		}
		if len(podInfos) == 0 {
			// fmt.Println("No pod for Milvus ", component)
			continue
		}
		fmt.Println("Pod Status:")
		for _, info := range podInfos {
			err = PrintPodStatus(client, info)
			if err != nil {
				allErrs = append(allErrs, err)
			}
		}
		fmt.Println("")
	}
	return utilerrors.NewAggregate(allErrs)
}

func (o MilvusDescribeOptions) DescribeMilvusDependence(f cmdutil.Factory, client client.Client, dependence string, selector string) error {
	var err error
	allErrs := []error{}
	infos, err := o.GetResource(f, []string{"statefulset"}, selector)
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		return nil
	}
	fmt.Printf("Milvus Dependence Status (%s): \n", dependence)
	err = PrintStatefulSetStatus(client, infos[0])
	if err != nil {
		allErrs = append(allErrs, err)
	}
	podInfos, err := o.GetResource(f, []string{"pod"}, selector)
	if err != nil {
		allErrs = append(allErrs, err)
		return utilerrors.NewAggregate(allErrs)
	}
	if len(podInfos) == 0 {
		// fmt.Println("No pod found for ", dependence)
		return nil
	}
	fmt.Println("Pod Status:")
	for _, info := range podInfos {
		err = PrintPodStatus(client, info)
		if err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

func (o MilvusDescribeOptions) DescribePulsar(f cmdutil.Factory, client client.Client, selector string) error {
	var err error
	allErrs := []error{}
	infos, err := o.GetResource(f, []string{"statefulset"}, selector)
	if err != nil {
		return err
	}
	jobInfos, err := o.GetResource(f, []string{"job"}, selector)
	if err != nil {
		return err
	}
	if len(infos) == 0 && len(jobInfos) == 0 {
		return nil
	}

	fmt.Println("Milvus Dependence Status (pulsar):")
	for _, info := range infos {
		err = PrintStatefulSetStatus(client, info)
		if err != nil {
			allErrs = append(allErrs, err)
		}
		getPodCom := strings.Split(info.Name, "-")
		podCom := getPodCom[len(getPodCom)-1]
		podSelector := selector + ", component=" + podCom
		podInfos, err := o.GetResource(f, []string{"pod"}, podSelector)
		if err != nil {
			allErrs = append(allErrs, err)
			continue
		}
		if len(podInfos) == 0 {
			// fmt.Println("No pod for Milvus ", podCom)
			continue
		}
		fmt.Println("Pod Status:")
		for _, info := range podInfos {
			err = PrintPodStatus(client, info)
			if err != nil {
				allErrs = append(allErrs, err)
			}
		}
		fmt.Println("")
	}

	for _, info := range jobInfos {
		err = PrintJobStatus(client, info)
		if err != nil {
			allErrs = append(allErrs, err)
		}
		getPodCom := strings.Split(info.Name, "-")
		podCom := getPodCom[len(getPodCom)-2] + "-init"
		podSelector := selector + ", component=" + podCom
		podInfos, err := o.GetResource(f, []string{"pod"}, podSelector)
		if err != nil {
			allErrs = append(allErrs, err)
			continue
			// return utilerrors.NewAggregate(allErrs)
		}
		if len(podInfos) == 0 {
			fmt.Println("")
			continue
		}

		fmt.Println("Pod Status:")
		for _, info := range podInfos {
			err = PrintPodStatus(client, info)
			if err != nil {
				allErrs = append(allErrs, err)
			}
		}
		fmt.Println("")
	}
	return utilerrors.NewAggregate(allErrs)
}

func (o MilvusDescribeOptions) GetResource(f cmdutil.Factory, labels []string, selector string) ([]*resource.Info, error) {
	r := f.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(o.Namespace).
		LabelSelectorParam(selector).
		ResourceTypeOrNameArgs(true, labels...).
		RequestChunksOf(o.DescribeOptions.DescriberSettings.ChunkSize).
		Flatten().
		Do()
	if err := r.Err(); err != nil {
		return nil, err
	}
	infos, err := r.Infos()
	return infos, err
}

func PrintPodStatus(client client.Client, info *resource.Info) error {
	pod := &corev1.Pod{}
	err := client.Get(context.Background(), conclient.ObjectKey{
		Namespace: info.Namespace,
		Name:      info.Name,
	}, pod)

	if errors.IsNotFound(err) {
		return err
	}

	fmt.Println("  Name:           ", info.Name)
	fmt.Println("  Status:         ", pod.Status.Phase)
	fmt.Println("  Conditions:")
	fmt.Println("    Type:         ", pod.Status.Conditions[0].Type)
	fmt.Println("    Status:       ", pod.Status.Conditions[0].Status)
	fmt.Println("  Containers:")
	fmt.Println("    Name:         ", pod.Status.ContainerStatuses[0].Name)
	// fmt.Println("    State:        ", pod.Status.ContainerStatuses[0].State)
	fmt.Println("    Ready:        ", pod.Status.ContainerStatuses[0].Ready)
	fmt.Println("    RestartCount: ", pod.Status.ContainerStatuses[0].RestartCount)
	fmt.Println("    Started:      ", *pod.Status.ContainerStatuses[0].Started)
	fmt.Println("")

	return nil
}

func PrintDeploymentStatus(client client.Client, info *resource.Info) error {
	deployment := &appsv1.Deployment{}
	err := client.Get(context.Background(), conclient.ObjectKey{
		Namespace: info.Namespace,
		Name:      info.Name,
	}, deployment)

	if errors.IsNotFound(err) {
		return err
	}

	fmt.Println("Name:             ", info.Name)
	fmt.Println("Replicas:         ", deployment.Status.Replicas, " desired | ", deployment.Status.AvailableReplicas, " available | ", deployment.Status.UnavailableReplicas, " unavailable")
	if len(deployment.Status.Conditions) != 0 {
		fmt.Println("Conditions:     ")
		fmt.Println("  Type:         ", deployment.Status.Conditions[0].Type)
		fmt.Println("  Status:       ", deployment.Status.Conditions[0].Status)
	}

	return nil
}

func PrintStatefulSetStatus(client client.Client, info *resource.Info) error {
	statefulSet := &appsv1.StatefulSet{}

	err := client.Get(context.Background(), conclient.ObjectKey{
		Namespace: info.Namespace,
		Name:      info.Name,
	}, statefulSet)

	if errors.IsNotFound(err) {
		return err
	}

	fmt.Println("Name:             ", info.Name)
	fmt.Println("Replicas:         ", statefulSet.Status.Replicas, " desired | ", statefulSet.Status.ReadyReplicas, " ready")
	if len(statefulSet.Status.Conditions) != 0 {
		fmt.Println("Conditions:     ")
		fmt.Println("  Type:         ", statefulSet.Status.Conditions[0].Type)
		fmt.Println("  Status:       ", statefulSet.Status.Conditions[0].Status)
	}
	return nil
}

func PrintJobStatus(client client.Client, info *resource.Info) error {
	job := &batchv1.Job{}

	err := client.Get(context.Background(), conclient.ObjectKey{
		Namespace: info.Namespace,
		Name:      info.Name,
	}, job)

	if errors.IsNotFound(err) {
		return err
	}

	fmt.Println("Name:             ", info.Name)
	fmt.Println("Replicas:         ", job.Status.Active, " active | ", job.Status.Succeeded, " succeeded | ", job.Status.Failed, " failed")
	if len(job.Status.Conditions) != 0 {
		fmt.Println("Conditions:     ")
		fmt.Println("  Type:           ", job.Status.Conditions[0].Type)
		fmt.Println("  Status:         ", job.Status.Conditions[0].Status)
	}
	return nil
}

func PrintMilvusStatus(client client.Client, namespace string, name string) error {
	milvus := &v1beta1.Milvus{}

	err := client.Get(context.Background(), conclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, milvus)

	if errors.IsNotFound(err) {
		return err
	}

	fmt.Println("Name:             ", name)
	fmt.Println("Status:           ", milvus.Status.Status)
	fmt.Println("Endpoint:         ", milvus.Status.Endpoint)
	if len(milvus.Status.Conditions) != 0 {
		fmt.Println("Conditions:")
	}
	for _, condition := range milvus.Status.Conditions {
		fmt.Println("  Type:         ", condition.Type)
		fmt.Println("  Status:       ", condition.Status)
		fmt.Println("  Message:      ", condition.Message)
	}

	fmt.Println("Replicas:")
	replicas := milvus.Status.Replicas
	if replicas.Proxy != 0 {
		fmt.Println("  Proxy:        ", replicas.Proxy)
	}
	if replicas.MixCoord != 0 {
		fmt.Println("  MixCoord:     ", replicas.MixCoord)
	}
	if replicas.RootCoord != 0 {
		fmt.Println("  RootCoord:    ", replicas.RootCoord)
	}
	if replicas.DataCoord != 0 {
		fmt.Println("  DataCoord:    ", replicas.DataCoord)
	}
	if replicas.IndexCoord != 0 {
		fmt.Println("  IndexCoord:   ", replicas.IndexCoord)
	}
	if replicas.QueryCoord != 0 {
		fmt.Println("  QueryCoord:   ", replicas.QueryCoord)
	}
	if replicas.DataNode != 0 {
		fmt.Println("  DataNode:     ", replicas.DataNode)
	}
	if replicas.IndexNode != 0 {
		fmt.Println("  IndexNode:    ", replicas.IndexNode)
	}
	if replicas.QueryNode != 0 {
		fmt.Println("  QueryNode:    ", replicas.QueryNode)
	}
	if replicas.Standalone != 0 {
		fmt.Println("  Standalone:   ", replicas.Standalone)
	}
	return nil
}

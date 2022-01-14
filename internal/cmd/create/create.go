package create

import (
	"context"
	"fmt"
	"github.com/milvus-io/milvus-operator/apis/milvus.io/v1alpha1"
	pkgerr "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectlcreate "k8s.io/kubectl/pkg/cmd/create"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"log"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)
var (
	createLong = templates.LongDesc(i18n.T(`
		The create subcommand installs the milvus version like standalone or cluster in the cluster
    `))
)
type printFn func(format string, v ...interface{})
type MilvusCreateOptions struct {
	Mode string
	Type string
	Values []string
	Namespace string
	CreateOptions *kubectlcreate.CreateOptions
	ResouceSetting map[string]interface{}
}
func NewMivlusCreateOptions(ioStreams genericclioptions.IOStreams) *MilvusCreateOptions {
	return &MilvusCreateOptions{
		Type: "",
		Mode: "",
		Namespace: "default",
		CreateOptions: kubectlcreate.NewCreateOptions(ioStreams),
	}
}
func NewMilvusCreateCmd(f cmdutil.Factory, ioStreams genericclioptions.IOStreams,client *client.Client) *cobra.Command {
	o := NewMivlusCreateOptions(ioStreams)
	createCmd := &cobra.Command{
		Use: "create {-f filename | -t type -m model}",
		Short: "create milvuse in kubernetes cluster",
		Long: createLong,
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(o.CreateOptions.FilenameOptions.Filenames) > 0 && (o.Mode != "" || o.Type != ""){
				ioStreams.ErrOut.Write([]byte("Error:-f conflict with other flag, if you want to specify filename,it can't set another flag"))
				os.Exit(1)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			//if cmdutil.IsFilenameSliceEmpty(o.CreateOptions.FilenameOptions.Filenames,o.CreateOptions.FilenameOptions.Kustomize) {
			//	ioStreams.ErrOut.Write([]byte("Error: must specify one of -f and -k\\n\\n"))
			//}.
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.ValidateArgs(cmd, args))
			cmdutil.CheckErr(o.Run(f, cmd,client))
		},
	}
	o.CreateOptions.RecordFlags.AddFlags(createCmd)

	usage := "to use to create the resouce"
	cmdutil.AddFilenameOptionFlags(createCmd,&o.CreateOptions.FilenameOptions,usage)
	cmdutil.AddValidateFlags(createCmd)
	o.CreateOptions.PrintFlags.AddFlags(createCmd)
	cmdutil.AddApplyAnnotationFlags(createCmd)
	cmdutil.AddDryRunFlag(createCmd)

	createCmd.Flags().StringVarP(&o.Mode,"mode","m",o.Mode,"use mode parameter to choose milvus standalone or cluster")
	createCmd.Flags().StringVarP(&o.Namespace,"namespace","n",o.Namespace,"use type parameter to choose install namespace")
	createCmd.Flags().StringVarP(&o.Type,"type","t",o.Type,"use type parameter to choose milvus cluster minimal,medium or large")
	createCmd.Flags().StringArrayVar(&o.Values,"set",[]string{},"the resource requirement requests for milvus cluster")
	_ = createCmd.MarkFlagRequired("mode")
	
	return createCmd
}

func (o *MilvusCreateOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error
	err = o.CreateOptions.Complete(f,cmd)
	if err != nil {
		return err
	}
	return err
}
func (o *MilvusCreateOptions) ValidateArgs(cmd *cobra.Command,args []string) error{
	var err error
	err = o.CreateOptions.ValidateArgs(cmd,args)
	if err != nil {
		return err
	}
	if len(o.Values)  > 0 {
		base := map[string]interface{}{}
		for _,value := range o.Values {
			if err := strvals.ParseInto(value, base); err != nil {
				return  pkgerr.Wrap(err, "failed parsing --set data")
			}
		}
		o.ResouceSetting = base
	}
	return err
}
func (o *MilvusCreateOptions) Run(f cmdutil.Factory, cmd *cobra.Command,client *client.Client) error {
	var err error
	if len(o.CreateOptions.FilenameOptions.Filenames) > 0 {
		err = o.CreateOptions.RunCreate(f,cmd)
		if err != nil {
			return err
		}
	}
	if o.Mode == "cluster" {
		

	}
	if o.Mode == "standalone" {
		if _,err = o.newMilvusStandalone(*client,context.TODO());err != nil {
			return err
		}
	}
	return err
}
//func runSetMilvusConfig(o *MilvusCreateOptions, milvus *v1alpha1.Milvus) error {
//	if len(o.Values) == 0 {
//		return nil
//	}
//	for key, vaylue :=  range o.ResouceSetting {
//		tags := strings.Split(key,".")
//	}
//	milvus.Spec.
//	return nil
//}

func (o *MilvusCreateOptions)newMilvusStandalone(client client.Client,ctx context.Context) (*v1alpha1.Milvus,error) {
	log.Printf("Creating the milvus in default namespace")
	milvus := &v1alpha1.Milvus{}
	switch o.Type {
	case "minimal":
		namespacedName := types.NamespacedName{
			Name: "milvus",
			Namespace: o.Namespace,
		}
		err := client.Get(ctx,namespacedName,milvus)
		if errors.IsNotFound(err) {
			newMilvus := &v1alpha1.Milvus{
				ObjectMeta:metav1.ObjectMeta{
					Name:"milvus",
					Namespace: o.Namespace,
				},
			}
			newMilvus.Spec,err = mapCoverToMilvusStandaloneSpec(coalesceValues(log.Printf,reflectToMap(milvus.Spec),o.ResouceSetting))
			if err != nil {
				return nil,err
			}
			err = client.Create(ctx,newMilvus)
			if err != nil{
				return nil ,err
			}
		}else{
			milvus.Spec,err = mapCoverToMilvusStandaloneSpec(coalesceValues(log.Printf,reflectToMap(milvus.Spec),o.ResouceSetting))
			if err != nil {
				return nil,err
			}
			err = client.Update(ctx,milvus)
			if err != nil{
				return nil ,err
			}
		}
	case "medium":
	case "large":
		
	}
	return milvus,nil
}
func newMilvusCluster(client client.Client,ctx context.Context,Type string) (*v1alpha1.MilvusCluster,error){
	log.Printf("Creating the milvus cluster in default namespace")
	switch Type {
	case "minimal":
	case "medium":
	case "large":
	}
	namespacedName := types.NamespacedName{
		Name: "milvus-cluster",
		Namespace: "default",
	}
	milvusCluster := &v1alpha1.MilvusCluster{}
	err := client.Get(ctx,namespacedName,milvusCluster)
	if errors.IsNotFound(err) {
		milvusCluster = &v1alpha1.MilvusCluster{
			ObjectMeta:metav1.ObjectMeta{
				Name:"milvus",
				Namespace: "default",
			},
		}
		milvusCluster.Spec = v1alpha1.MilvusClusterSpec{
			Dep: v1alpha1.MilvusClusterDependencies{

			},
			Com:v1alpha1.MilvusComponents{

			},
			Conf: v1alpha1.Values{

			},
		}
	}
	err = client.Create(ctx,milvusCluster)
	if err != nil {
		return nil,err
	}
	return milvusCluster,nil
}

// coalesceValues builds up a values map for a particular CRD.
//
// Values in v will override the src values in the crd.
func coalesceValues(printf printFn,src,values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return src
	}
	for key,val := range src {
		if value,ok := values[key]; ok {
			if value == nil {
				delete(values,key)
			}else if  dest,ok := value.(map[string]interface{});ok {
				src,ok := val.(map[string]interface{})
				if !ok {
					if val != nil {
						printf("warning:skipped value for %s.%s: Not a table.")
					}
				}else {
					coalesceTablesFullKey(printf, dest, src)

				}
			}
		}else{
			values[key] = val
		}

	}
	return values
}
// coalesceTablesFullKey merges a source map into a destination map.
//
// dest is considered authoritative.
func coalesceTablesFullKey(printf printFn, dst, src map[string]interface{}) map[string]interface{} {
	// When --reuse-values is set but there are no modifications yet, return new values
	if src == nil {
		return dst
	}
	if dst == nil {
		return src
	}
	// Because dest has higher precedence than src, dest values override src
	// values.
	for key, val := range src {
		if dv, ok := dst[key]; ok && dv == nil {
			delete(dst, key)
		} else if !ok {
			dst[key] = val
		} else if istable(val) {
			if istable(dv) {
				coalesceTablesFullKey(printf, dv.(map[string]interface{}), val.(map[string]interface{}))
			} else {
				printf("warning: cannot overwrite table with non table for %s (%v)", val)
			}
		} else if istable(dv) && val != nil {
			printf("warning: destination for %s is a table. Ignoring non-table value (%v)", val)
		}
	}
	fmt.Println(dst)
	return dst
}

// istable is a special-purpose function to see if the present thing matches the definition of a YAML table.
func istable(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}
func  reflectToMap(obj interface{}) map[string]interface{} {
	t := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)

	var data = make(map[string]interface{})
	for i := 0; i < t.NumField(); i++ {

		if v.Field(i).CanInterface() {

			//判断是否是嵌套结构
			if v.Field(i).Type().Kind() == reflect.Struct{
				if t.Field(i).Anonymous {
					mergeMap(data,reflectToMap(v.Field(i).Interface()))
				}else{
					data[readTag(string(t.Field(i).Tag.Get("json")))] = reflectToMap(v.Field(i).Interface())

				}
			}else{
				data[readTag(string(t.Field(i).Tag.Get("json")))] = v.Field(i).Interface()

			}
		}
	}
	return data

}


func mapCoverToMilvusStandaloneSpec(Map map[string]interface{}) (v1alpha1.MilvusSpec,error) {
	milvusSpec := v1alpha1.MilvusSpec{}
	instanceByte, err := json.Marshal(Map)
	if err != nil {
		return milvusSpec,err
	}

	err = json.Unmarshal(instanceByte,&milvusSpec)
	if err != nil {
		return milvusSpec,err
	}
	return milvusSpec,nil
}

func mapCoverToMilvusClusterSpec(Map map[string]interface{}) (*v1alpha1.MilvusClusterSpec,error) {
	instanceByte, err := json.Marshal(Map)
	if err != nil {
		return nil,err
	}
	milvusClusterSpec := v1alpha1.MilvusClusterSpec{}
	err = json.Unmarshal(instanceByte,&milvusClusterSpec)
	if err != nil {
		return nil,err
	}
	return &milvusClusterSpec,nil
}

func mergeMap(src,dest map[string]interface{}) {
	for i,v := range dest {
		src[i] = v
	}
}
func readTag( tag string) string {
	if tag != "" {
		res := strings.Split(tag, ",")
		if res[0] != "" {
			return res[0]
		}
	}
	return tag
}

package create

import (
	"context"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/milvus-io/milvus-operator/apis/milvus.io/v1beta1"
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
	// "log"
	"io/ioutil"
	"os"
	"path"
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
	Mode           string
	Type           string
	Values         []string
	Namespace      string
	CreateOptions  *kubectlcreate.CreateOptions
	ResouceSetting map[string]interface{}
}

func NewMivlusCreateOptions(ioStreams genericclioptions.IOStreams) *MilvusCreateOptions {
	return &MilvusCreateOptions{
		Type:          "",
		Mode:          "",
		Namespace:     "default",
		CreateOptions: kubectlcreate.NewCreateOptions(ioStreams),
	}
}
func NewMilvusCreateCmd(f cmdutil.Factory, ioStreams genericclioptions.IOStreams, client *client.Client) *cobra.Command {
	o := NewMivlusCreateOptions(ioStreams)
	createCmd := &cobra.Command{
		Use:   "create instance_name {-f filename | -t type -m model}",
		Short: "create milvus in kubernetes cluster",
		Long:  createLong,
		Args:  cobra.MaximumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(o.CreateOptions.FilenameOptions.Filenames) > 0 && (o.Mode != "" || o.Type != "") {
				ioStreams.ErrOut.Write([]byte("Error: -f conflict with other flag, if you want to specify filename,it can't set another flag"))
				os.Exit(1)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.ValidateArgs(cmd, args))
			cmdutil.CheckErr(o.Run(f, cmd, client, args))
		},
	}
	o.CreateOptions.RecordFlags.AddFlags(createCmd)

	usage := "to use to create the resouce"
	cmdutil.AddFilenameOptionFlags(createCmd, &o.CreateOptions.FilenameOptions, usage)
	cmdutil.AddValidateFlags(createCmd)
	o.CreateOptions.PrintFlags.AddFlags(createCmd)
	cmdutil.AddApplyAnnotationFlags(createCmd)
	cmdutil.AddDryRunFlag(createCmd)

	createCmd.Flags().StringVarP(&o.Mode, "mode", "m", o.Mode, "use mode parameter to choose milvus standalone or cluster")
	createCmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "use type parameter to choose install namespace")
	createCmd.Flags().StringVarP(&o.Type, "type", "t", o.Type, "use type parameter to choose milvus cluster minimal,medium or large")
	createCmd.Flags().StringArrayVar(&o.Values, "set", []string{}, "the resource requirement requests for milvus cluster")
	// _ = createCmd.MarkFlagRequired("mode")

	return createCmd
}

func (o *MilvusCreateOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	if err := o.CreateOptions.Complete(f, cmd); err != nil {
		return err
	}
	return nil
}
func (o *MilvusCreateOptions) ValidateArgs(cmd *cobra.Command, args []string) error {
	if len(o.Values) > 0 {
		base := map[string]interface{}{}
		for _, value := range o.Values {
			if err := strvals.ParseInto(value, base); err != nil {
				return pkgerr.Wrap(err, "failed parsing --set data")
			}
		}
		o.ResouceSetting = base
	}
	return nil
}
func (o *MilvusCreateOptions) Run(f cmdutil.Factory, cmd *cobra.Command, client *client.Client, args []string) error {
	if len(o.CreateOptions.FilenameOptions.Filenames) > 0 {
		if err := o.CreateOptions.RunCreate(f, cmd); err != nil {
			return err
		}
		return nil
	}

	if len(args) != 1 {
		return cmdutil.UsageErrorf(cmd, "accepts 1 arg(s), received %v", len(args))
	}

	if _, err := o.newMilvusInstance(*client, context.TODO(), args[0]); err != nil {
		return err
	}
	return nil
}

func (o *MilvusCreateOptions) newMilvusInstance(client client.Client, ctx context.Context, instanceName string) (*v1beta1.Milvus, error) {
	namespacedName := types.NamespacedName{
		Name:      instanceName,
		Namespace: o.Namespace,
	}

	if !errors.IsNotFound(client.Get(ctx, namespacedName, &v1beta1.Milvus{})) {
		return nil, fmt.Errorf("Error: milvuses.milvus.io %s already exists", instanceName)
	}

	if o.Mode != "cluster" && o.Mode != "" && o.Mode != "standalone" {
		return nil, fmt.Errorf("Error mode, please specify one of the following modes: 'standalone', 'cluster'")
	}

	newMilvus := &v1beta1.Milvus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceName,
			Namespace: o.Namespace,
		},
	}

	switch o.Type {
	case "minimal":
		if o.Mode == "standalone" {
			spec, err := yamlToObj("minimal_standalone.yaml")
			if err != nil {
				return nil, err
			}
			newMilvus.Spec = *spec
		} else {
			spec, err := yamlToObj("minimal_cluster.yaml")
			if err != nil {
				return nil, err
			}
			newMilvus.Spec = *spec
		}
	case "medium":
		if o.Mode == "standalone" {
			spec, err := yamlToObj("medium_standalone.yaml")
			if err != nil {
				return nil, err
			}
			newMilvus.Spec = *spec
		} else {
			spec, err := yamlToObj("medium_cluster.yaml")
			if err != nil {
				return nil, err
			}
			newMilvus.Spec = *spec
		}
	case "large":
		if o.Mode == "standalone" {
			spec, err := yamlToObj("large_standalone.yaml")
			if err != nil {
				return nil, err
			}
			newMilvus.Spec = *spec
		} else {
			spec, err := yamlToObj("large_cluster.yaml")
			if err != nil {
				return nil, err
			}
			newMilvus.Spec = *spec
		}
	default:
		return nil, fmt.Errorf("Error type, please specify one of the following types: 'minimal', 'medium', 'large'")
	}

	err := parsingNestedStructure(reflect.ValueOf(&newMilvus.Spec).Elem(), o.ResouceSetting)
	if err != nil {
		return nil, err
	}
	// fmt.Println("Dest Milvus cluster spec", newMilvusCluster.Spec)
	if err = client.Create(ctx, newMilvus); err != nil {
		return nil, err
	}
	return newMilvus, nil
}

func yamlToObj(fileName string) (*v1beta1.MilvusSpec, error) {
	filePath := path.Join("deploy", fileName)
	var milvusSpec v1beta1.MilvusSpec
	ymlSpec, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	jsSpec, err := yaml.YAMLToJSON([]byte(ymlSpec))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(jsSpec, &milvusSpec); err != nil {
		return nil, err
	}
	return &milvusSpec, nil
}

func parsingNestedStructure(v reflect.Value, values map[string]interface{}) error {
	var err error

	if values == nil {
		return err
	}

	if v.Type().String() == "v1beta1.Values" {
		// newValues := reflect.ValueOf(values).Convert(v.FieldByName("Data").Type())
		// fmt.Println("source value: ", v.FieldByName("Data"))
		// fmt.Println("new value: ", values)
		// v.FieldByName("Data").Set(newValues)
		// err := mapOverwirte(v.FieldByName("Data"), values)
		v.FieldByName("Data").Set(reflect.ValueOf(values).Convert(v.FieldByName("Data").Type()))
		return err
	}

	tagMap := getTagMap(v.Type())

	for key, value := range values {
		field, ok := tagMap[key]
		if ok {
			subSpec := v.FieldByName(field)
			setValue(subSpec, value)
		} else if _, ok := tagMap[",inline"]; ok {
			if tagMap[",inline"] == "ComponentSpec" {
				if field, ok := ifTagInSpec(reflect.TypeOf(v1beta1.ComponentSpec{}), key); ok {
					subSpec := v.FieldByName("ComponentSpec").FieldByName(field)
					setValue(subSpec, value)
				} else {
					err = fmt.Errorf("The field %q does not exists", key)
				}
			} else if tagMap[",inline"] == "Component" {
				if field, ok := ifTagInSpec(reflect.TypeOf(v1beta1.Component{}), key); ok {
					subSpec := v.FieldByName("Component").FieldByName(field)
					setValue(subSpec, value)
				} else if field, ok := ifTagInSpec(reflect.TypeOf(v1beta1.ComponentSpec{}), key); ok {
					subSpec := v.FieldByName("Component").FieldByName("ComponentSpec").FieldByName(field)
					setValue(subSpec, value)
				} else {
					err = fmt.Errorf("The field %q does not exists", key)
				}
			} else if tagMap[",inline"] == "ServiceComponent" {
				if field, ok := ifTagInSpec(reflect.TypeOf(v1beta1.ServiceComponent{}), key); ok {
					subSpec := v.FieldByName("ServiceComponent").FieldByName(field)
					setValue(subSpec, value)
				} else if field, ok := ifTagInSpec(reflect.TypeOf(v1beta1.Component{}), key); ok {
					subSpec := v.FieldByName("ServiceComponent").FieldByName("Component").FieldByName(field)
					setValue(subSpec, value)
				} else if field, ok := ifTagInSpec(reflect.TypeOf(v1beta1.ComponentSpec{}), key); ok {
					subSpec := v.FieldByName("ServiceComponent").FieldByName("Component").FieldByName("ComponentSpec").FieldByName(field)
					setValue(subSpec, value)
				} else {
					err = fmt.Errorf("The field %q does not exists", key)
				}
			}
		} else {
			err = fmt.Errorf("The field %q does not exists", key)
		}
	}
	return err
}

// Overwrite the Milvus Spev with the setting values
func setValue(subSpec reflect.Value, value interface{}) error {
	var err error
	if subSpec.Kind() == reflect.Struct {
		if istable(value) {
			parsingNestedStructure(subSpec, value.(map[string]interface{}))
		} else {
			err = fmt.Errorf("Can not be overwritten with value %q", value)
		}
	} else if subSpec.Kind() == reflect.Ptr {
		if subSpec.IsNil() {
			newSpec := reflect.New(subSpec.Type().Elem())
			if newSpec.Type().Elem().Kind() == reflect.Struct {
				if istable(value) {
					parsingNestedStructure(newSpec.Elem(), value.(map[string]interface{}))
				} else {
					err = fmt.Errorf("Can not be overwritten with value %q", value)
				}
			} else {
				setValue(newSpec.Elem(), value)
			}
			subSpec.Set(newSpec)
		} else {
			if subSpec.Type().Elem().Kind() == reflect.Struct {
				if istable(value) {
					parsingNestedStructure(subSpec.Elem(), value.(map[string]interface{}))
				} else {
					err = fmt.Errorf("Can not be overwritten with value %q", value)
				}
			} else {
				setValue(subSpec.Elem(), value)
			}
		}
	} else {
		subSpec.Set(reflect.ValueOf(value).Convert(subSpec.Type()))
	}
	return err
}

// func mapOverwirte(v reflect.Value, values map[string]interface{}) error {
// 	for key, value := range values {

// 	}
// }

func ifTagInSpec(t reflect.Type, key string) (string, bool) {
	tagMap := getTagMap(t)
	if field, ok := tagMap[key]; ok {
		return field, true
	} else {
		return "", false
	}
}

func getTagMap(t reflect.Type) map[string]string {
	tagMap := make(map[string]string)
	var tag string
	for i := 0; i < t.NumField(); i++ {
		tag = readTag(t.Field(i).Tag.Get("json"))
		tagMap[tag] = t.Field(i).Name
	}
	return tagMap
}

// istable is a special-purpose function to see if the present thing matches the definition of a YAML table.
func istable(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

func readTag(tag string) string {
	if tag != "" {
		res := strings.Split(tag, ",")
		if res[0] != "" {
			return res[0]
		}
	}
	return tag
}

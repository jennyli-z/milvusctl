package update

import (
	"context"
	"fmt"
	"github.com/milvus-io/milvus-operator/apis/milvus.io/v1beta1"
	pkgerr "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	// "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectlapply "k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	// "k8s.io/kubectl/pkg/util/i18n"
	// "k8s.io/kubectl/pkg/util/templates"
	// "log"
	// "os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type printFn func(format string, v ...interface{})

type MilvusUpdateOptions struct {
	Mode           string
	Type           string
	Values         []string
	Namespace      string
	ApplyOptions   *kubectlapply.ApplyOptions
	ResouceSetting map[string]interface{}
}

func NewMilvusUpdateOptions(ioStreams genericclioptions.IOStreams) *MilvusUpdateOptions {
	return &MilvusUpdateOptions{
		Type:         "",
		Mode:         "",
		Namespace:    "default",
		ApplyOptions: kubectlapply.NewApplyOptions(ioStreams),
	}
}

func NewMilvusUpdateCmd(f cmdutil.Factory, ioStreams genericclioptions.IOStreams, client *client.Client) *cobra.Command {
	o := NewMilvusUpdateOptions(ioStreams)
	// o.ApplyOptions.cmdBaseName = baseName

	updateCmd := &cobra.Command{
		Use:   "update instance_name {-f filename | -t type -m model --set [options]}",
		Short: "update milvus instance in kubernetes cluster",
		Long:  "The update subcommand updates the milvus configuration",
		Args:  cobra.MaximumNArgs(1),
		// PreRun: func(cmd *cobra.Command, args []string) {
		// 	if o.FileName != "" && (o.Mode != "" || o.Type != "") {
		// 		ioStreams.ErrOut.Write([]byte("Error: -f conflict with other flag, if you want to specify filename,it can't set another flag"))
		// 		os.Exit(1)
		// 	}
		// },

		Run: func(cmd *cobra.Command, args []string) {
			// cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.validateArgs(cmd, args))
			cmdutil.CheckErr(o.validatePruneAll(o.ApplyOptions.Prune, o.ApplyOptions.All, o.ApplyOptions.Selector))
			cmdutil.CheckErr(o.Run(f, cmd, client, args))
		},
	}

	o.ApplyOptions.DeleteFlags.AddFlags(updateCmd)
	o.ApplyOptions.RecordFlags.AddFlags(updateCmd)
	o.ApplyOptions.PrintFlags.AddFlags(updateCmd)

	cmdutil.AddValidateFlags(updateCmd)
	cmdutil.AddDryRunFlag(updateCmd)
	cmdutil.AddServerSideApplyFlags(updateCmd)
	cmdutil.AddFieldManagerFlagVar(updateCmd, &o.ApplyOptions.FieldManager, "kubectl-client-side-apply")

	updateCmd.Flags().StringVarP(&o.Mode, "mode", "m", o.Mode, "use mode parameter to choose milvus standalone or cluster")
	updateCmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "use type parameter to choose install namespace")
	updateCmd.Flags().StringVarP(&o.Type, "type", "t", o.Type, "use type parameter to choose milvus cluster minimal,medium or large")
	// updateCmd.Flags().StringVarP(&o.FileName, "filename", "f", o.FileName, "use type parameter to choose milvus cluster minimal,medium or large")
	updateCmd.Flags().StringArrayVar(&o.Values, "set", []string{}, "the resource requirement requests for milvus cluster")
	// _ = updateCmd.MarkFlagRequired("mode")

	return updateCmd
}

func (o *MilvusUpdateOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error
	err = o.ApplyOptions.Complete(f, cmd)
	if err != nil {
		return err
	}
	return nil
}

func (o *MilvusUpdateOptions) validateArgs(cmd *cobra.Command, args []string) error {
	// if len(args) != 0 {
	// 	return cmdutil.UsageErrorf(cmd, "Unexpected args: %v", args)
	// }
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

func (o *MilvusUpdateOptions) validatePruneAll(prune, all bool, selector string) error {
	if all && len(selector) > 0 {
		return fmt.Errorf("cannot set --all and --selector at the same time")
	}
	if prune && !all && selector == "" {
		return fmt.Errorf("all resources selected for prune without explicitly passing --all. To prune all resources, pass the --all flag. If you did not mean to prune all resources, specify a label selector")
	}
	return nil
}

func (o *MilvusUpdateOptions) Run(f cmdutil.Factory, cmd *cobra.Command, client *client.Client, args []string) error {

	// namespacedName := types.NamespacedName{
	// 	Name:      instanceName,
	// 	Namespace: o.Namespace,
	// }
	// if errors.IsNotFound(client.Get(ctx, namespacedName, &v1alpha1.Milvus{})) {
	// 	return nil, fmt.Errorf("Error: milvuses.milvus.io %s do not exists in namespace: %s", instanceName, o.Namespace)
	// }

	if len(args) == 0 {
		cmdutil.CheckErr(o.Complete(f, cmd))
		if err := o.ApplyOptions.Run(); err != nil {
			return err
		}
		return nil
	}

	if len(args) != 1 {
		return fmt.Errorf("accepts 1 arg(s), received %v", len(args))
	}

	if _, err := o.updateMilvusInstance(*client, context.TODO(), args[0]); err != nil {
		return err
	}

	return nil
}

func (o *MilvusUpdateOptions) updateMilvusInstance(client client.Client, ctx context.Context, instanceName string) (*v1beta1.Milvus, error) {
	// log.Printf("Creating the milvus in default namespace")
	// var err error
	milvus := &v1beta1.Milvus{}
	namespacedName := types.NamespacedName{
		Name:      instanceName,
		Namespace: o.Namespace,
	}
	if errors.IsNotFound(client.Get(ctx, namespacedName, milvus)) {
		return nil, fmt.Errorf("milvuses.milvus.io %s do not exists in namespace: %s", instanceName, o.Namespace)
	}

	err := parsingNestedStructure(reflect.ValueOf(&milvus.Spec).Elem(), o.ResouceSetting)

	if err != nil {
		return nil, err
	}

	// fmt.Println("Dest spec: ", milvus.Spec)

	err = client.Update(ctx, milvus)
	if err != nil {
		return nil, err
	} else {
		fmt.Printf("milvus.milvus.io/%s Update \n", instanceName)
	}

	return milvus, nil
}

func parsingNestedStructure(v reflect.Value, values map[string]interface{}) error {
	var err error

	if values == nil {
		return err
	}

	if v.Type().String() == "v1beta1.Values" {
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

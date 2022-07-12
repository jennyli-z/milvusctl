package delete

import (
	"context"
	"github.com/milvus-io/milvus-operator/apis/milvus.io/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectldelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MilvusDeleteOptions struct {
	WithDeletions   bool
	Namespace       string
	Deleteflags     *kubectldelete.DeleteFlags
	DeletionOptions *kubectldelete.DeleteOptions
}

func NewMivlusDeleteOptions(ioStreams genericclioptions.IOStreams) *MilvusDeleteOptions {
	deletflags := kubectldelete.NewDeleteFlags("containing the milvus  to delete.")
	o, _ := deletflags.ToOptions(nil, ioStreams)
	return &MilvusDeleteOptions{
		Deleteflags:     deletflags,
		WithDeletions:   false,
		Namespace:       "default",
		DeletionOptions: o,
	}
}
func NewMilvusDeleteCmd(f cmdutil.Factory, ioStreams genericclioptions.IOStreams, client *client.Client) *cobra.Command {
	o := NewMivlusDeleteOptions(ioStreams)
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "delete milvus in kubernetes cluster",
		Long:  "The deelte subcommand uninstalls the milvus version like standalone or cluster in the cluster",
		PreRun: func(cmd *cobra.Command, args []string) {

		},
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run(client))
		},
	}
	o.Deleteflags.AddFlags(deleteCmd)
	cmdutil.AddDryRunFlag(deleteCmd)
	deleteCmd.Flags().BoolVar(&o.WithDeletions, "with-deletion", o.WithDeletions, "automatically add pvc deletion parameter on deletion")
	deleteCmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "use type parameter to choose delete namespace")

	return deleteCmd
}
func (o *MilvusDeleteOptions) Run(client *client.Client) error {
	ctx := context.Background()
	if err := o.deleteMilvus(*client, ctx); err != nil {
		return err
	}
	if err := o.deleteMilvusCluster(*client, ctx); err != nil {
		return err
	}
	return nil
}
func (o *MilvusDeleteOptions) deleteMilvus(client client.Client, ctx context.Context) error {
	var milvus v1alpha1.Milvus
	namespacedName := types.NamespacedName{
		Name:      "milvus",
		Namespace: o.Namespace,
	}
	err := client.Get(ctx, namespacedName, &milvus)
	if errors.IsNotFound(err) {
		return nil
	}
	if o.WithDeletions == true {
		milvus.Spec.Dep.Etcd.InCluster.PVCDeletion = true
		milvus.Spec.Dep.Etcd.InCluster.DeletionPolicy = "Delete"
		milvus.Spec.Dep.Storage.InCluster.PVCDeletion = true
		milvus.Spec.Dep.Storage.InCluster.DeletionPolicy = "Delete"
		if err = client.Update(ctx, &milvus); err != nil {
			return err
		}
	}
	err = client.Delete(ctx, &milvus)
	if err != nil {
		return err
	}
	return nil
}
func (o *MilvusDeleteOptions) deleteMilvusCluster(client client.Client, ctx context.Context) error {
	var mlc v1alpha1.MilvusCluster
	namespacedName := types.NamespacedName{
		Name:      "milvuscluster",
		Namespace: o.Namespace,
	}
	err := client.Get(ctx, namespacedName, &mlc)
	if errors.IsNotFound(err) {
		return nil
	}
	if o.WithDeletions == true {
		mlc.Spec.Dep.Etcd.InCluster.PVCDeletion = true
		mlc.Spec.Dep.Etcd.InCluster.DeletionPolicy = "Delete"
		mlc.Spec.Dep.Storage.InCluster.PVCDeletion = true
		mlc.Spec.Dep.Storage.InCluster.DeletionPolicy = "Delete"
		mlc.Spec.Dep.Pulsar.InCluster.PVCDeletion = true
		mlc.Spec.Dep.Pulsar.InCluster.DeletionPolicy = "Delete"

		if err = client.Update(ctx, &mlc); err != nil {
			return err
		}
	}
	err = client.Delete(ctx, &mlc)
	if err != nil {
		return err
	}
	return nil
}

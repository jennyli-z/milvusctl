package pkg

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateMilvusOperatorSecert(ctx context.Context,data map[string]string,client client.Client) error {
	log.Printf("Creating the milvus-operator secert")
	namespacedName := types.NamespacedName{
		Name:      "milvusctl-milvus-operator",
		Namespace: "milvus-operator",
	}
	configmap := &corev1.ConfigMap{}
	fmt.Println(client)
	err := client.Get(ctx,namespacedName,configmap)
	if errors.IsNotFound(err) {
		configmap = &corev1.ConfigMap{
			ObjectMeta:metav1.ObjectMeta{
				Name: "milvusctl-milvus-operator",
				Namespace: "milvus-operator",
			},
		}
		configmap.Data = data
		return client.Create(ctx,configmap)
	}else if err != nil{
		return err
	}
	return nil
}
func DeleteMilvusOperatorSecert(ctx context.Context,client client.Client) error {
	log.Printf("Deleting the milvus-operator secert")

	namespacedName := types.NamespacedName{
		Name:      "milvusctl-milvus-operator",
		Namespace: "milvus-operator",
	}
	var configMap = new (corev1.ConfigMap)
	if err := client.Get(ctx, namespacedName, configMap); err != nil {
		return err
	} else {
		return client.Delete(ctx,configMap)
	}
	return nil
}
func FetchDataFromSecret(ctx context.Context,client client.Client) (map[string]string,error) {
	log.Printf("Fetching the milvus-operator secert data")
	namespacedName := types.NamespacedName{
		Name:      "milvusctl-milvus-operator",
		Namespace: "milvus-operator",
	}
	fmt.Println(client)
	var configMap = new(corev1.ConfigMap)
	if err := client.Get(ctx, namespacedName, configMap); err != nil {
		fmt.Println(client)
		fmt.Println(namespacedName)
		fmt.Println("can't find the configmap")
		return nil,err
	} else {
		return configMap.Data,err
	}
	return nil,nil
}
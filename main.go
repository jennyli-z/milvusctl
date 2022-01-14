/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"fmt"
	"github.com/milvus-io/milvus-operator/apis/milvus.io/v1alpha1"
	"github.com/milvus-io/milvusctl/internal/cmd"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"io/ioutil"
	"log"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
	"strings"
)
var (
	settings = cli.New()
)

func debug(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}
func main() {
	cfg := new(action.Configuration)
	var client client.Client
	rootCmd := cmd.NewMilvusCmd(cfg,&client)

	cobra.OnInitialize(func() {
		helmDriver := os.Getenv("HELM_DRIVER")
		if err := cfg.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, debug); err != nil {
			log.Fatal(err)
		}
		if helmDriver == "memory" {
			loadReleasesInMemory(cfg)
		}
		client = loadClientset()

	})
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// This function loads releases into the memory storage if the
// environment variable is properly set.
func loadReleasesInMemory(actionConfig *action.Configuration) {
	filePaths := strings.Split(os.Getenv("HELM_MEMORY_DRIVER_DATA"), ":")
	if len(filePaths) == 0 {
		return
	}

	store := actionConfig.Releases
	mem, ok := store.Driver.(*driver.Memory)
	if !ok {
		// For an unexpected reason we are not dealing with the memory storage driver.
		return
	}

	actionConfig.KubeClient = &kubefake.PrintingKubeClient{Out: ioutil.Discard}

	for _, path := range filePaths {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatal("Unable to read memory driver data", err)
		}

		releases := []*release.Release{}
		if err := yaml.Unmarshal(b, &releases); err != nil {
			log.Fatal("Unable to unmarshal memory driver data: ", err)
		}

		for _, rel := range releases {
			if err := store.Create(rel); err != nil {
				log.Fatal(err)
			}
		}
	}
	// Must reset namespace to the proper one
	mem.SetNamespace(settings.Namespace())
}

func loadClientset() client.Client {
	cfg, _ := config.GetConfig()
	client, _ := client.New(cfg, client.Options{})
	scheme := client.Scheme()
	v1alpha1.AddToScheme(scheme)
	return client
}
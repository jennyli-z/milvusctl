package pkg

import (
	"context"
	"fmt"
	"github.com/jetstack/cert-manager/cmd/ctl/pkg/install/helm"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"log"
	"os"
)

type InstallOptions struct {
	Settings  *cli.EnvSettings
	Client    *action.Install
	Cfg       *action.Configuration
	ValueOpts *values.Options

	ChartName string
	DryRun    bool
	Wait      bool
}

const (
	installCRDsFlagName         = "installCRDs"
	defaultCertManagerNamespace = "cert-manager"
	defaultReleaseName          = "cert-manager"
)

func (o *InstallOptions) RunInstall(ctx context.Context) (*release.Release, error) {
	//if _,err := o.Cfg.Releases.History(defaultReleaseName); err  == nil {
	//	return nil,err
	//}
	// Find chart
	cp, err := o.Client.ChartPathOptions.LocateChart(o.ChartName, o.Settings)
	if err != nil {
		fmt.Println("find cert-manager chart faild: ", err)
		return nil, err
	}

	chart, err := loader.Load(cp)
	if err != nil {
		fmt.Println("load cert-manager chart faild: ", err)
		return nil, err
	}

	// Check if chart is installable
	if err := checkIfInstallable(chart); err != nil {
		fmt.Println("check cert-manager chart faild: ", err)
		return nil, err
	}

	// Console print if chart is deprecated
	if chart.Metadata.Deprecated {
		log.Printf("This chart is deprecated")
	}

	// Merge all values flags
	p := getter.All(o.Settings)
	chartValues, err := o.ValueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	// Dryrun template generation (used for rendering the CRDs in /templates)
	o.Client.DryRun = true                  // Do not apply install
	o.Client.ClientOnly = true              // Do not validate against cluster (otherwise double CRDs can cause error)
	chartValues[installCRDsFlagName] = true // Make sure to render CRDs
	dryRunResult, err := o.Client.Run(chart, chartValues)
	if err != nil {
		return nil, err
	}

	if o.DryRun {
		return dryRunResult, nil
	}

	if err := o.Cfg.Init(o.Settings.RESTClientGetter(), o.Settings.Namespace(), os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, err
	}

	// Extract the resource.Info objects from the manifest
	resources, err := helm.ParseMultiDocumentYAML(dryRunResult.Manifest, o.Cfg.KubeClient)
	if err != nil {
		return nil, err
	}

	// Filter resource.Info objects and only keep the CRDs
	crds := helm.FilterCrdResources(resources)

	// Abort in case CRDs were not found in chart
	if len(crds) == 0 {
		return nil, fmt.Errorf("Found no CRDs in provided cert-manager chart.")
	}

	// Make sure that no CRDs are currently installed
	originalCRDs, err := helm.FetchResources(crds, o.Cfg.KubeClient)
	if err != nil {
		return nil, err
	}

	if len(originalCRDs) == 0 {
		// Install CRDs
		if err := helm.CreateCRDs(crds, o.Cfg); err != nil {
			return nil, err
		}

	}

	// Install chart
	o.Client.DryRun = false     // Apply DryRun cli flags
	o.Client.ClientOnly = false // Perform install against cluster

	o.Client.Wait = o.Wait // Wait for resources to be ready
	// If part of the install fails and the Atomic option is set to True,
	// all resource installs are reverted. Atomic cannot be enabled without
	// waiting (if Atomic=True is set, the value for Wait is overwritten with True),
	// so only enable Atomic if we are waiting.
	o.Client.Atomic = o.Wait
	// The cert-manager chart currently has only a startupapicheck hook,
	// if waiting is disabled, this hook should be disabled too; otherwise
	// the hook will still wait for the installation to succeed.
	o.Client.DisableHooks = !o.Wait
	o.Client.Replace = true
	o.Client.CreateNamespace = true
	//o.client.Namespace = defaultCertManagerNamespace
	chartValues[installCRDsFlagName] = false // Do not render CRDs, as this might cause problems when uninstalling using helm

	return o.Client.Run(chart, chartValues)
}

// Only Application chart type are installable.
func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return fmt.Errorf("%s charts are not installable", ch.Metadata.Type)
}

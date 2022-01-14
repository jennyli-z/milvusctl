package pkg

import (
	"context"
	"helm.sh/helm/v3/pkg/action"
)

type UnInstallOptions struct {
	Client    *action.Uninstall
	Cfg       *action.Configuration
}
func (o *UnInstallOptions) RunUninstall(ctx context.Context) error {
	_,err := o.Cfg.Releases.History(defaultReleaseName)
	if err != nil {
		return err
	}
	o.Client.DisableHooks = true
	_,err = o.Client.Run(defaultReleaseName)
	if err != nil {
		return err
	}
	return nil
}
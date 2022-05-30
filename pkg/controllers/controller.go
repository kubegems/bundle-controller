/*
Copyright 2022 The kubegems.io Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controllers

import (
	"context"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	bundlecommon "kubegems.io/bundle-controller/pkg/apis/bundle"
	"kubegems.io/bundle-controller/pkg/bundle"

	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme        = runtime.NewScheme()
	setupLog      = ctrl.Log.WithName("setup")
	leaseDuration = 30 * time.Second
	renewDeadline = 20 * time.Second
)

// nolint: gochecknoinits
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(bundlev1.SchemeBuilder.AddToScheme(scheme))
}

type Options struct {
	MetricsAddr          string `json:"metricsAddr,omitempty" description:"The address the metric endpoint binds to."`
	ProbeAddr            string `json:"probeAddr,omitempty" description:"The address the probe endpoint binds to."`
	EnableLeaderElection bool   `json:"enableLeaderElection,omitempty" description:"Enable leader election for controller manager."`
	SearchDir            string `json:"searchDir,omitempty" description:"The directory to search for bundle manifests."`
}

func NewDefaultOptions() *Options {
	return &Options{
		MetricsAddr:          ":9090",
		ProbeAddr:            ":8081",
		EnableLeaderElection: false,
		SearchDir:            "bundles",
	}
}

func Run(ctx context.Context, options *Options, bundleoptions *bundle.Options) error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     options.MetricsAddr,
		HealthProbeBindAddress: options.ProbeAddr,
		LeaseDuration:          &leaseDuration,
		RenewDeadline:          &renewDeadline,
		LeaderElection:         options.EnableLeaderElection,
		LeaderElectionID:       bundlecommon.GroupName,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		return err
	}

	// setup healthz
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	// setup controllers
	if err := Setup(mgr, bundleoptions); err != nil {
		setupLog.Error(err, "unable to set up helm controller")
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}

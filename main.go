// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/spf13/pflag"
	runv1alpha3 "github.com/vmware-tanzu/tanzu-framework/apis/run/v1alpha3"
	akov1beta1 "github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/pkg/apis/ako/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	akoov1alpha1 "github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/api/v1alpha1"
	"github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/controllers"
	ako_operator "github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/pkg/ako-operator"
)

var (
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	logOptions           = logs.NewOptions()
	metricsAddr          string
	enableLeaderElection bool
	profilerAddress      string
)

func initLog() {
	ctrl.SetLogger(klog.Background())
}

func init() {
	initLog()
	// ignoring errors
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = akoov1alpha1.AddToScheme(scheme)
	_ = akov1beta1.AddToScheme(scheme)
	_ = runv1alpha3.AddToScheme(scheme)
}

// InitFlags initializes the flags.
func InitFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(logOptions, fs)

	fs.StringVar(&metricsAddr, "metrics-addr", "localhost:8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&profilerAddress, "profiler-addr", "", "Bind address to expose the pprof profiler")
}

func main() {

	InitFlags(pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	// Set log level 2 as default.
	if err := pflag.CommandLine.Set("v", "2"); err != nil {
		setupLog.Error(err, "Failed to set default log level")
		os.Exit(1)
	}
	pflag.Parse()

	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	if profilerAddress != "" {
		setupLog.Info(
			"Profiler listening for requests",
			"profiler-addr", profilerAddress)
		go runProfiler(profilerAddress)
	}

	// Default webserver port is 9443.
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Scheme:         scheme,
		LeaderElection: enableLeaderElection,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	err = controllers.SetupReconcilers(mgr)
	if err != nil {
		setupLog.Error(err, "Unable to setup reconcilers")
		os.Exit(1)
	}

	printRunningEnv()

	//setup webhook here
	if err = (&akoov1alpha1.AKODeploymentConfig{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "AKODeploymentConfig")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func runProfiler(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		setupLog.Error(err, "unable to start listening")
	}
}

func printRunningEnv() {
	if ako_operator.IsBootStrapCluster() {
		setupLog.Info("AKO Operator Running in Bootstrap Kind Cluster")
	} else {
		setupLog.Info("AKO Operator Running in Management Cluster")
	}

	if ako_operator.IsClusterClassEnabled() {
		setupLog.Info("AKO Operator Running in Cluster Class Based Cluster")
	} else {
		setupLog.Info("AKO Operator Running in Legacy Cluster")
	}
}

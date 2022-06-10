// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"

	"github.com/pkg/errors"
	ako_operator "github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/ako-operator"
	"github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/handlers"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	akoov1alpha1 "github.com/vmware-samples/load-balancer-operator-for-kubernetes/api/v1alpha1"
	"github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/haprovider"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SetupWithManager adds this reconciler to a new controller then to the
// provided manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Watch Cluster resources.
		For(&clusterv1.Cluster{}).
		Watches(
			&source.Kind{Type: &corev1.Service{}},
			handler.EnqueueRequestsFromMapFunc(handlers.ClusterForService(r.Client, r.Log)),
		).
		Complete(r)
}

type ClusterReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Haprovider *haprovider.HAProvider
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := r.Log.WithValues("Cluster", req.NamespacedName)

	res := ctrl.Result{}
	// Get the resource for this request.
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Cluster not found, will not reconcile")
			return res, nil
		}
		return res, err
	}

	// Always Patch when exiting this function so changes to the resource are updated on the API server.
	patchHelper, err := patch.NewHelper(cluster, r.Client)
	if err != nil {
		return res, errors.Wrapf(err, "failed to init patch helper for %s %s",
			cluster.GroupVersionKind(), req.NamespacedName)
	}
	defer func() {
		if err := patchHelper.Patch(ctx, cluster); err != nil {
			if reterr == nil {
				reterr = err
			}
			log.Error(err, "patch failed")
		}
	}()

	log = log.WithValues("Cluster", cluster.Namespace+"/"+cluster.Name)

	if ako_operator.IsHAProvider() {
		log.Info("AVI is control plane HA provider")
		r.Haprovider = haprovider.NewProvider(r.Client, r.Log)
		if err = r.Haprovider.CreateOrUpdateHAService(ctx, cluster); err != nil {
			log.Error(err, "Fail to reconcile HA service")
			return res, err
		}
		if ako_operator.IsBootStrapCluster() {
			return res, nil
		}
	}

	akoDeploymentConfig, err := ako_operator.GetAKODeploymentConfigForCluster(ctx, r.Client, log, cluster)
	if err != nil {
		log.Error(err, "failed to get cluster matched akodeploymentconfig")
		return res, err
	}

	if akoDeploymentConfig == nil {
		// Removing finalizer if current cluster can't be selected by any akoDeploymentConfig
		log.Info("Not find cluster matched akodeploymentconfig, removing finalizer", "finalizer", akoov1alpha1.ClusterFinalizer)
		ctrlutil.RemoveFinalizer(cluster, akoov1alpha1.ClusterFinalizer)
		// Removing avi label after deleting all the resources
		delete(cluster.Labels, akoov1alpha1.AviClusterLabel)
		log.Info("Cluster doesn't have AVI enabled, skip Cluster reconciling")
		return res, nil
	}

	log.Info("Cluster has AVI enabled")
	return res, nil
}

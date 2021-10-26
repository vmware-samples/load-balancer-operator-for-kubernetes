// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

// This packages provides utilities to interact with AKO

package ako

import (
	"context"

	"github.com/go-logr/logr"
	akoov1alpha1 "github.com/vmware-samples/load-balancer-operator-for-kubernetes/api/v1alpha1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	akoStatefulSetName = "ako"
	akoConditionType   = appv1.StatefulSetConditionType("ako.vmware.com/ObjectDeletionInProgress")
)

func CleanupFinished(ctx context.Context, remoteClient client.Client, log logr.Logger) (bool, error) {

	ss := &appv1.StatefulSet{}
	if err := remoteClient.Get(ctx, client.ObjectKey{
		Name:      akoStatefulSetName,
		Namespace: akoov1alpha1.AviNamespace,
	}, ss); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("AKO Statefulset is gone, consider it as a signal as deletion has finished")
			return true, nil
		}
		log.Error(err, "Failed to get AKO StatefulSet")
		return false, err
	}

	return conditionHasStatus(ss.Status.Conditions, akoConditionType, corev1.ConditionFalse), nil
}

func conditionHasStatus(conditions []appv1.StatefulSetCondition, ctype appv1.StatefulSetConditionType, status corev1.ConditionStatus) bool {
	for _, c := range conditions {
		if c.Type == ctype && c.Status == status {
			return true
		}
	}
	return false
}

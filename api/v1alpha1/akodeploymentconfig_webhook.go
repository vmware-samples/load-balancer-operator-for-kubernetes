// Copyright 2022 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var akoDeploymentConfigLog = logf.Log.WithName("akodeploymentconfig-resource")

func (r *AKODeploymentConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-networking-tkg-tanzu-vmware-com-v1alpha1-akodeploymentconfig,mutating=true,failurePolicy=fail,groups=networking.tkg.tanzu.vmware.com,resources=akodeploymentconfigs,verbs=create;update,versions=v1alpha1,name=vakodeploymentconfig.kb.io,sideEffects=None,admissionReviewVersions=v1;v1alpha1

//+kubebuilder:webhook:verbs=create;update;delete,path=/validate-networking-tkg-tanzu-vmware-com-v1alpha1-akodeploymentconfig,mutating=false,failurePolicy=fail,groups=networking.tkg.tanzu.vmware.com,resources=akodeploymentconfigs,versions=v1alpha1,name=vakodeploymentconfig.kb.io, sideEffects=None, admissionReviewVersions=v1;v1alpha1

var _ webhook.Validator = &AKODeploymentConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AKODeploymentConfig) ValidateCreate() error {
	akoDeploymentConfigLog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList
	selector, err := metav1.LabelSelectorAsSelector(&r.Spec.ClusterSelector)
	if err != nil {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "ClusterSelector"), r.Spec.ClusterSelector, err.Error()),
		)
	}

	// non default ADC (a.k.a name is not install-ako-for-all), should have non-empty cluster selector
	if r.ObjectMeta.Name != WorkloadClusterAkoDeploymentConfig && selector.Empty() {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "ClusterSelector"), r.Spec.ClusterSelector, "field should not be empty for non-default ADC"))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("AKODeploymentConfig").GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AKODeploymentConfig) ValidateUpdate(old runtime.Object) error {
	var allErrs field.ErrorList

	akoDeploymentConfigLog.Info("validate update", "name", r.Name)

	oldADC, ok := old.(*AKODeploymentConfig)

	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a AKODeploymentConfig but got a %T", old))
	}

	if oldADC != nil {
		if (oldADC.Spec.ControlPlaneNetwork.CIDR != r.Spec.ControlPlaneNetwork.CIDR) || (oldADC.Spec.ControlPlaneNetwork.Name != r.Spec.ControlPlaneNetwork.Name) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "ControlPlaneNetwork"), r.Spec.ControlPlaneNetwork, "field should not be changed"))
		}
	}

	if oldADC != nil {
		if oldADC.Spec.ClusterSelector.String() != r.Spec.ClusterSelector.String() {
			allErrs = append(
				allErrs,
				field.Invalid(field.NewPath("spec", "ClusterSelector"), r.Spec.ClusterSelector, "field should not be changed"))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("AKODeploymentConfig").GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AKODeploymentConfig) ValidateDelete() error {
	akoDeploymentConfigLog.Info("validate delete", "name", r.Name)
	return nil
}

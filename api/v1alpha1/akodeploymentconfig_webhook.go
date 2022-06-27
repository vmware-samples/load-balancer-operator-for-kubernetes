// Copyright 2022 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"bytes"
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/pkg/aviclient"
)

// log is for logging in this package.
var akoDeploymentConfigLog = logf.Log.WithName("akodeploymentconfig-resource")
var kclient client.Client
var aviClient aviclient.Client
var runTest bool

func (r *AKODeploymentConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {
	kclient = mgr.GetClient()
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
	allErrs = append(allErrs, r.validateClusterSelector(nil)...)
	allErrs = append(allErrs, r.validateAVI(nil)...)
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("AKODeploymentConfig").GroupKind(), r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AKODeploymentConfig) ValidateUpdate(old runtime.Object) error {
	akoDeploymentConfigLog.Info("validate update", "name", r.Name)
	oldADC, ok := old.(*AKODeploymentConfig)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a AKODeploymentConfig but got a %T", old))
	}
	var allErrs field.ErrorList
	if oldADC != nil {
		allErrs = append(allErrs, r.validateClusterSelector(oldADC)...)
		allErrs = append(allErrs, r.validateAVI(oldADC)...)
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

// validateClusterSelector checks AKODeploymentConfig object's cluster selector field input is valid or not
// when old is nil, it is used for AKODeploymentConfig object create, otherwise it is used for AKODeploymentConfig
// object update
func (r *AKODeploymentConfig) validateClusterSelector(old *AKODeploymentConfig) field.ErrorList {
	var allErrs field.ErrorList
	// when update AKODeploymentConfig object, cluster selector should be immutable
	if old != nil {
		if old.Spec.ClusterSelector.String() != r.Spec.ClusterSelector.String() {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "ClusterSelector"),
				r.Spec.ClusterSelector,
				"field should not be changed"))
			return allErrs
		}
	}
	// convert cluster selector to label selector
	selector, err := metav1.LabelSelectorAsSelector(&r.Spec.ClusterSelector)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "ClusterSelector"),
			r.Spec.ClusterSelector,
			err.Error()))
	}
	// non default ADC (a.k.a name is not install-ako-for-all), should have non-empty cluster selector
	if r.ObjectMeta.Name != WorkloadClusterAkoDeploymentConfig && selector.Empty() {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "ClusterSelector"),
			r.Spec.ClusterSelector,
			"field should not be empty for non-default ADC"))
	}
	return allErrs
}

// validateAVI checks all NSX Advanced Load Balancer related fields are valid or not
// when old is nil, it is used for AKODeploymentConfig object create, otherwise it is used for AKODeploymentConfig
// object update. Following fields are already required fileds in CRD, so no need to check if those fields are empty.
// - adminCredentialRef
// - certificateAuthorityRef
// - cloudName
// - controller
// - dataNetwork
// - serviceEngineGroup
func (r *AKODeploymentConfig) validateAVI(old *AKODeploymentConfig) field.ErrorList {
	var allErrs field.ErrorList

	// check avi related secret
	adminCredential := &corev1.Secret{}
	if err := r.validateAviSecret(adminCredential, r.Spec.AdminCredentialRef); err != nil {
		allErrs = append(allErrs, err)
	}

	aviControllerCA := &corev1.Secret{}
	if err := r.validateAviSecret(aviControllerCA, r.Spec.CertificateAuthorityRef); err != nil {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) != 0 {
		return allErrs
	}

	controlerVersion := "20.1.3"
	if r.Spec.ControllerVersion != "" {
		controlerVersion = r.Spec.ControllerVersion
	}

	if !runTest {
		client, err := aviclient.NewAviClient(&aviclient.AviClientConfig{
			ServerIP: r.Spec.Controller,
			Username: string(adminCredential.Data["username"][:]),
			Password: string(adminCredential.Data["password"][:]),
			CA:       string(aviControllerCA.Data["certificateAuthorityData"][:]),
		}, controlerVersion)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "Controller"), r.Spec.Controller, "failed to init avi client for controller:"+err.Error()))
			return allErrs
		}
		aviClient = client
	}

	if old == nil {
		// when old is nil, it is creating a new AKODeploymentConfig object, check following fields
		if err := r.validateAviCloud(); err != nil {
			allErrs = append(allErrs, err)
		}
		if err := r.validateAviServiceEngineGroup(); err != nil {
			allErrs = append(allErrs, err)
		}
		if err := r.validateAviControlPlaneNetworks(); err != nil {
			allErrs = append(allErrs, err...)
		}
		if err := r.validateAviDataNetworks(); err != nil {
			allErrs = append(allErrs, err...)
		}
	} else {
		// when old is not nil, it is updating an existing AKODeploymentConfig object,
		// only check changed fields
		if old.Spec.CloudName != r.Spec.CloudName {
			if err := r.validateAviCloud(); err != nil {
				allErrs = append(allErrs, err)
			}
		}
		if old.Spec.ServiceEngineGroup != r.Spec.ServiceEngineGroup {
			if err := r.validateAviServiceEngineGroup(); err != nil {
				allErrs = append(allErrs, err)
			}
		}
		// control plane network should be immutable since cluster control plane endpoint
		// can't be updated
		if (old.Spec.ControlPlaneNetwork.Name != r.Spec.ControlPlaneNetwork.Name) ||
			(old.Spec.ControlPlaneNetwork.CIDR != r.Spec.ControlPlaneNetwork.CIDR) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "ControlPlaneNetwork"),
				r.Spec.ControlPlaneNetwork,
				"field should not be changed"))
		}
		if (old.Spec.DataNetwork.Name != r.Spec.DataNetwork.Name) ||
			(old.Spec.DataNetwork.CIDR != r.Spec.DataNetwork.CIDR) {
			if err := r.validateAviDataNetworks(); err != nil {
				allErrs = append(allErrs, err...)
			}
		}
	}
	return allErrs
}

// validateAviSecret checks NSX Advanced Load Balancer related credentials or certificate secret is valid or not
func (r *AKODeploymentConfig) validateAviSecret(secret *corev1.Secret, secretRef SecretReference) *field.Error {
	if err := kclient.Get(context.Background(), client.ObjectKey{
		Name:      secretRef.Name,
		Namespace: secretRef.Namespace,
	}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return field.Invalid(field.NewPath("spec", secretRef.Namespace+"/"+secretRef.Name),
				secretRef.Name,
				"can't find secret")
		} else {
			return field.Invalid(field.NewPath("spec", secretRef.Namespace+"/"+secretRef.Name),
				secretRef.Name,
				"failed to find secret:"+err.Error())
		}
	}
	return nil
}

// validateAviCloud checks input Cloud Name field valid or not
func (r *AKODeploymentConfig) validateAviCloud() *field.Error {
	if cloud, err := aviClient.CloudGetByName(r.Spec.CloudName); err != nil {
		return field.Invalid(field.NewPath("spec", "cloudName"), r.Spec.CloudName,
			"failed to get cloud from avi controller:"+err.Error())
	} else if cloud.IPAMProviderRef == nil {
		return field.Invalid(field.NewPath("spec", "cloudName"), r.Spec.CloudName,
			"this cloud doesn't have any ipam profile configured")
	}
	return nil
}

// validateAviServiceEngineGroup checks input Servcie Engine Group valid or not
func (r *AKODeploymentConfig) validateAviServiceEngineGroup() *field.Error {
	if _, err := aviClient.ServiceEngineGroupGetByName(r.Spec.ServiceEngineGroup); err != nil {
		return field.Invalid(field.NewPath("spec", "serviceEngineGroup"), r.Spec.ServiceEngineGroup,
			"failed to get service engine group from avi controller:"+err.Error())
	}
	return nil
}

// validateAviControlPlaneNetworks checks input Control Plane Network name existing or not, CIDR format valid or not
func (r *AKODeploymentConfig) validateAviControlPlaneNetworks() field.ErrorList {
	var allErrs field.ErrorList
	// check control plane network name
	if _, err := aviClient.NetworkGetByName(r.Spec.ControlPlaneNetwork.Name); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "controlPlaneNetwork", "name"),
			r.Spec.ControlPlaneNetwork.Name,
			"failed to get control plane network "+r.Spec.ControlPlaneNetwork.Name+" from avi controller:"+err.Error()))
	}
	// check network cidr validate or not
	_, _, err := net.ParseCIDR(r.Spec.ControlPlaneNetwork.CIDR)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "controlPlaneNetwork", "cidr"),
			r.Spec.ControlPlaneNetwork.CIDR,
			"control plane network cidr "+r.Spec.ControlPlaneNetwork.CIDR+" is not valid:"+err.Error()))
	}
	return allErrs
}

// validateAviDataNetworks checks input
// Data Plane Network name existing or not
// CIDR format valid or not
// IPPools format valid or not
func (r *AKODeploymentConfig) validateAviDataNetworks() field.ErrorList {
	var allErrs field.ErrorList
	// check data network name
	if _, err := aviClient.NetworkGetByName(r.Spec.DataNetwork.Name); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dataNetwork", "name"),
			r.Spec.DataNetwork.Name,
			"failed to get data plane network "+r.Spec.DataNetwork.Name+" from avi controller:"+err.Error()))
	}
	// check network cidr
	_, cidr, err := net.ParseCIDR(r.Spec.DataNetwork.CIDR)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dataNetwork", "cidr"),
			r.Spec.DataNetwork.CIDR,
			"data plane network cidr "+r.Spec.DataNetwork.CIDR+" is not valid:"+err.Error()))
	}
	// check data network ip pools
	for _, ipPool := range r.Spec.DataNetwork.IPPools {
		ipStart := net.ParseIP(ipPool.Start)
		ipEnd := net.ParseIP(ipPool.End)
		if ipStart == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dataNetwork", "ipPools"),
				r.Spec.DataNetwork.IPPools,
				"ip pool address"+ipPool.Start+" is not valid"))
		}
		if ipEnd == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dataNetwork", "ipPools"),
				r.Spec.DataNetwork.IPPools,
				"ip pool address"+ipPool.End+" is not valid"))
		}
		if !cidr.Contains(ipStart) || !cidr.Contains(ipEnd) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dataNetwork", "ipPools"),
				r.Spec.DataNetwork.IPPools,
				"Range ["+ipPool.Start+","+ipPool.End+"] is not in cidr"))
		}
		if bytes.Compare(ipStart, ipEnd) > 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dataNetwork", "ipPools"),
				r.Spec.DataNetwork.IPPools,
				ipPool.Start+" is greater than "+ipPool.End))
		}
		// TODO:(xudongl) will wait for AKO support v6 address to uncomment this
		// if ippool.Type != addrType {
		// 	return field.Invalid(field.NewPath("spec", "dataNetwork", "ipPools"),
		// 		r.Spec.DataNetwork.IPPools,
		// 		"data plane network ip pools type is not aligned with cidr")
		// }
	}
	return allErrs
}

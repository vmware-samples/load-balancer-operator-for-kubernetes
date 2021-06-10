// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// AKODeploymentConfigSpec defines the desired state of an AKODeploymentConfig
// AKODeploymentConfig describes the shared configurations for AKO deployments across a set
// of Clusters.
type AKODeploymentConfigSpec struct {
	// CloudName speficies the AVI Cloud AKO will be deployed with
	CloudName string `json:"cloudName"`

	// Controller is the AVI Controller endpoint to which AKO talks to
	// provision Load Balancer resources
	// The format is [scheme://]address[:port]
	// * scheme                     http or https, defaults to https if not
	//                              specified
	// * address                    IP address of the AVI Controller
	//                              specified
	// * port                       if not specified, use default port for
	//                              the corresponding scheme
	Controller string `json:"controller"`

	// ServiceEngineGroup is the group name of Service Engine that's to be used by the set
	// of AKO Deployments
	ServiceEngineGroup string `json:"serviceEngineGroup"`

	// Label selector for Clusters. The Clusters that are
	// selected by this will be the ones affected by this
	// AKODeploymentConfig.
	// It must match the Cluster labels. This field is immutable.
	// +optional
	ClusterSelector metav1.LabelSelector `json:"clusterSelector,omitempty"`

	// WorkloadCredentialRef points to a Secret resource which includes the username
	// and password to access and configure the Avi Controller.
	//
	// * username                   Username used with basic authentication for
	//                              the Avi REST API
	// * password                   Password used with basic authentication for
	//                              the Avi REST API
	//
	// This field is optional. When it's not specified, username/password
	// will be automatically generated for each Cluster and Tenant needs to
	// be non-nil in this case.
	// +optional
	WorkloadCredentialRef SecretReference `json:"workloadCredentialRef,omitempty"`

	// AdminCredentialRef points to a Secret resource which includes the username
	// and password to access and configure the Avi Controller.
	//
	// * username                   Username used with basic authentication for
	//                              the Avi REST API
	// * password                   Password used with basic authentication for
	//                              the Avi REST API
	//
	// This credential needs to be bound with admin tenant and will be used
	// by AKO Operator to automate configurations and operations.
	AdminCredentialRef SecretReference `json:"adminCredentialRef"`

	// CertificateAuthorityRef points to a Secret resource that includes the
	// AVI Controller's CA
	//
	// * certificateAuthorityData   PEM-encoded certificate authority
	//                              certificates
	//
	CertificateAuthorityRef SecretReference `json:"certificateAuthorityRef"`

	// The AVI tenant for the current AKODeploymentConfig
	// This field is optional.
	// +optional
	Tenant AVITenant `json:"tenant,omitempty"`

	// DataNetworks describes the Data Networks the AKO will be deployed
	// with.
	// This field is immutable.
	DataNetwork DataNetwork `json:"dataNetwork"`

	// ExtraConfigs contains extra configurations for AKO Deployment
	//
	// +optional
	ExtraConfigs ExtraConfigs `json:"extraConfigs,omitempty"`
}

// ExtraConfigs contains extra configurations for AKO Deployment
type ExtraConfigs struct {
	// Log specifies the configuration for AKO logging
	// +optional
	Log AKOLogConfig `json:"log,omitempty"`

	// Rbac specifies the configuration for AKO Rbac
	// +optional
	Rbac AKORbacConfig `json:"rbac,omitempty"`

	// IngressConfigs specifies ingress configuration for ako
	// +optional
	IngressConfigs AKOIngressConfig `json:"ingress,omitempty"`

	// DisableStaticRouteSync describes ako should sync static routing or not.
	// If the POD networks are reachable from the Avi SE, this should be to true.
	// Otherwise, it should be false.
	// It would be true by default.
	// +optional
	DisableStaticRouteSync bool `json:"disableStaticRouteSync,omitempty"`

	// CniPlugin describes which cni plugin cluster is using.
	// default value should be antrea, set this string if cluster cni is other type.
	// enum: calico|canal|flannel|openshift|antrea
	// +optional
	CniPlugin string `json:"cniPlugin,omitempty"`
}

// AKOIngressConfig contains ingress configurations for AKO Deployment
type AKOIngressConfig struct {
	// DisableIngressClass will prevent AKO Operator to install AKO
	// IngressClass into workload clusters for old version of K8s
	//
	// +optional
	DisableIngressClass bool `json:"disableIngressClass,omitempty"`

	// DefaultIngressController bool describes ako is the default
	// ingress controller to use
	//
	// +optional
	DefaultIngressController bool `json:"defaultIngressController,omitempty"`

	// ShardVSSize string describes ingress shared virtual service size
	// Valid value should be SMALL, MEDIUM or LARGE, default value is SMALL
	// +kubebuilder:validation:Enum=SMALL;MEDIUM;LARGE
	// +optional
	ShardVSSize string `json:"shardVSSize,omitempty"`

	// ServiceType string describes ingress methods for a service
	// Valid value should be NodePort or ClusterIP
	// +kubebuilder:validation:Enum=NodePort;ClusterIP
	// +optional
	ServiceType string `json:"serviceType,omitempty"`

	// NodeNetworkList describes the details of network and CIDRs
	// are used in pool placement network for vcenter cloud. Node Network details
	// are not needed when in NodePort mode / static routes are disabled / non vcenter clouds.
	// +optional
	NodeNetworkList []NodeNetwork `json:"nodeNetworkList,omitempty"`
}

type NodeNetwork struct {
	// NetworkName is the name of this network
	// +optional
	NetworkName string `json:"networkName,omitempty"`
	// Cidrs represents all the IP CIDRs in this network
	// +optional
	Cidrs []string `json:"cidrs,omitempty"`
}

type AKOLogConfig struct {
	// PersistentVolumeClaim specifies if a PVC should make for AKO logging
	// +optional
	PersistentVolumeClaim string `json:"persistentVolumeClaim,omitempty"`

	// MountPath specifies the path to mount PVC
	// +optional
	MountPath string `json:"mountPath,omitempty"`

	// LogFile specifies the log file name
	// +optional
	LogFile string `json:"logFile,omitempty"`
}

type AKORbacConfig struct {
	// PspPolicyAPIVersion decides the API version of the PodSecurityPolicy
	PspPolicyAPIVersion string `json:"pspPolicyAPIVersion,omitempty"`

	// PspEnabled enables the deployment of a PodSecurityPolicy that grants
	// AKO the proper role
	// +optional
	PspEnabled bool `json:"pspEnabled,omitempty"`
}

// AVITenant describes settings for an AVI Tenant object
type AVITenant struct {
	// Context is the type of AVI tenant context. Defaults to Provider. This field is immutable.
	// +kubebuilder:validation:Enum=Provider;Tenant
	Context string `json:"context,omitempty"`

	// Name is the name of the tenant. This field is immutable.
	Name string `json:"name"`
}

// DataNetwork describes one AVI Data Network
type DataNetwork struct {
	Name    string   `json:"name"`
	CIDR    string   `json:"cidr"`
	IPPools []IPPool `json:"ipPools,omitempty"`
}

// IPPool defines a contiguous range of IP Addresses
type IPPool struct {
	// Start represents the starting IP address of the pool.
	Start string `json:"start"`
	// End represents the ending IP address of the pool.
	End string `json:"end"`
	// Type represents the type of IP Address
	// +kubebuilder:validation:Enum=V4;
	Type string `json:"type"`
}

// SecretReference pointer to SecretRef
type SecretReference *SecretRef

// SecretRef references a Kind Secret object in the same kubernetes
// cluster
type SecretRef struct {
	// Name is the name of resource being referenced.
	Name string `json:"name"`
	// Namespace of the resource being referenced.
	Namespace string `json:"namespace"`
}

// AKODeploymentConfigStatus defines the observed state of AKODeploymentConfig
type AKODeploymentConfigStatus struct {
	// ObservedGeneration reflects the generation of the most recently
	// observed AKODeploymentConfig.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current state of the AKODeploymentConfig.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=akodeploymentconfigs,scope=Cluster
// +kubebuilder:subresource:status

// AKODeploymentConfig is the Schema for the akodeploymentconfigs API
type AKODeploymentConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AKODeploymentConfigSpec   `json:"spec,omitempty"`
	Status AKODeploymentConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AKODeploymentConfigList contains a list of AKODeploymentConfig
type AKODeploymentConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AKODeploymentConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AKODeploymentConfig{}, &AKODeploymentConfigList{})
}

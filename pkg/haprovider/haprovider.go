// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package haprovider

import (
	"context"
	"errors"
	"net"
	"sync"

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	akoov1alpha1 "github.com/vmware-samples/load-balancer-operator-for-kubernetes/api/v1alpha1"
	ako_operator "github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/ako-operator"
	akov1alpha1 "github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/pkg/apis/ako/v1alpha1"
)

type HAProvider struct {
	client.Client
	log logr.Logger
}

var instance *HAProvider
var once sync.Once
var QueryFQDN = queryFQDNEndpoint

// NewProvider make HAProvider as a singleton
func NewProvider(c client.Client, log logr.Logger) *HAProvider {
	once.Do(func() {
		instance = &HAProvider{
			Client: c,
			log:    log,
		}
	})
	return instance
}

func (r *HAProvider) getHAServiceName(cluster *clusterv1.Cluster) string {
	return cluster.Namespace + "-" + cluster.Name + "-" + akoov1alpha1.HAServiceName
}

func (r *HAProvider) CreateOrUpdateHAService(ctx context.Context, cluster *clusterv1.Cluster) error {
	serviceName := r.getHAServiceName(cluster)
	service := &corev1.Service{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      serviceName,
		Namespace: cluster.Namespace,
	}, service); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info(serviceName + "service doesn't exist, start creating it...")
			service, err = r.createService(ctx, cluster)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if err := r.updateClusterControlPlaneEndpoint(cluster, service); err != nil {
		return err
	}
	if _, err := r.ensureEndpoints(ctx, serviceName, service.Namespace); err != nil {
		return err
	}
	return nil
}

func (r *HAProvider) createService(
	ctx context.Context,
	cluster *clusterv1.Cluster,
) (*corev1.Service, error) {
	serviceName := r.getHAServiceName(cluster)

	serviceAnnotations, err := r.annotateService(ctx, cluster)
	if err != nil {
		return nil, err
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "core/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   cluster.Namespace,
			Annotations: serviceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Protocol:   "TCP",
				Port:       ako_operator.GetControlPlaneEndpointPort(),
				TargetPort: intstr.FromInt(int(6443)),
			},
			},
		},
	}
	// Add Finalizer on Management Cluster's service to avoid being deleted.
	if cluster.Namespace == akoov1alpha1.TKGSystemNamespace {
		ctrlutil.AddFinalizer(service, akoov1alpha1.HAServiceBootstrapClusterFinalizer)
	} else {
		service.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: cluster.APIVersion,
				Kind:       cluster.Kind,
				Name:       cluster.Name,
				UID:        cluster.UID,
			},
		}
	}
	if endpoint, ok := cluster.ObjectMeta.Annotations[akoov1alpha1.ClusterControlPlaneAnnotations]; ok {
		// "endpoint" can be ipv4 or hostname, add ipv4 or hostname to service.Spec.LoadBalancerIP
		if net.ParseIP(endpoint) == nil {
			endpoint, err = QueryFQDN(endpoint)
			if err != nil {
				r.log.Error(err, "Failed to resolve control plane endpoint ", "endpoint", endpoint)
				return nil, err
			}
		}
		service.Spec.LoadBalancerIP = endpoint
	}
	r.log.Info("Creating " + serviceName + " Service")
	err = r.Create(ctx, service)
	return service, err
}

func (r *HAProvider) annotateService(ctx context.Context, cluster *clusterv1.Cluster) (map[string]string, error) {
	serviceAnnotation := map[string]string{
		akoov1alpha1.HAServiceAnnotationsKey:  "true",
		akoov1alpha1.TKGClusterNameLabel:      cluster.Name,
		akoov1alpha1.TKGClusterNameSpaceLabel: cluster.Namespace,
	}

	if ako_operator.IsBootStrapCluster() {
		return serviceAnnotation, nil
	}

	adcForCluster, err := r.getADCForCluster(ctx, cluster)
	if err != nil {
		return serviceAnnotation, err
	}
	//no adc is selected for cluster, no annotation is needed.
	if adcForCluster == nil {
		return serviceAnnotation, nil
	}

	aviInfraSetting, err := r.getAviInfraSettingFromAdc(ctx, adcForCluster)
	if err != nil {
		return serviceAnnotation, err
	}

	if _, ok := cluster.Labels[akoov1alpha1.TKGManagememtClusterRoleLabel]; ok {
		if adcForCluster.Spec.ControlPlaneNetwork.CIDR != "" && adcForCluster.Spec.ControlPlaneNetwork.CIDR != adcForCluster.Spec.DataNetwork.CIDR {
			if aviInfraSetting == nil {
				return serviceAnnotation, errors.New("management cluster control plane network set, but corresponding AVIInfraSetting not found, requeue to wait for AVIInfraSetting created")
			}
		}
	}
	if aviInfraSetting != nil {
		//add AVIInfraSetting annotation when creating HA svc
		serviceAnnotation[akoov1alpha1.HAAVIInfraSettingAnnotationsKey] = aviInfraSetting.Name
	}
	return serviceAnnotation, nil
}

func (r *HAProvider) getADCForCluster(ctx context.Context, cluster *clusterv1.Cluster) (*akoov1alpha1.AKODeploymentConfig, error) {
	adcForCluster, err := ako_operator.GetAKODeploymentConfigForCluster(ctx, r.Client, r.log, cluster)
	if err != nil {
		return nil, err
	}
	if adcForCluster == nil {
		r.log.Info("Current cluster is not selected by any akoDeploymentConfig, skip adding AviInfraSetting annotation")
	}
	return adcForCluster, nil
}

func (r *HAProvider) getAviInfraSettingFromAdc(ctx context.Context, adcForCluster *akoov1alpha1.AKODeploymentConfig) (*akov1alpha1.AviInfraSetting, error) {

	aviInfraSetting := &akov1alpha1.AviInfraSetting{}
	aviInfraSettingName := GetAviInfraSettingName(adcForCluster)
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name: aviInfraSettingName,
	}, aviInfraSetting); err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info(aviInfraSettingName + " not found, skip adding annotation to HA service...")
			return nil, nil
		} else {
			r.log.Error(err, "Failed to get AVIInfraSetting, requeue")
			return nil, err
		}
	} else {
		return aviInfraSetting, nil
	}
}

func (r *HAProvider) updateClusterControlPlaneEndpoint(cluster *clusterv1.Cluster, service *corev1.Service) error {
	endpoint, _ := cluster.Annotations[akoov1alpha1.ClusterControlPlaneAnnotations]
	// Dakar Limitation: customers ensure the service engine is running
	ingress := service.Status.LoadBalancer.Ingress
	if len(ingress) > 0 && net.ParseIP(ingress[0].IP) != nil {
		if endpoint != "" && net.ParseIP(endpoint) == nil {
			cluster.Spec.ControlPlaneEndpoint.Host = endpoint
		} else {
			cluster.Spec.ControlPlaneEndpoint.Host = service.Status.LoadBalancer.Ingress[0].IP
		}
		cluster.Spec.ControlPlaneEndpoint.Port = ako_operator.GetControlPlaneEndpointPort()
		return nil
	}
	return errors.New(service.Name + " service external ip is not ready")
}

func (r *HAProvider) findEndpointInMachine(ip string, machine *clusterv1.Machine) bool {
	for _, machineAddress := range machine.Status.Addresses {
		if net.ParseIP(machineAddress.Address) != nil && ip == machineAddress.Address {
			return true
		}
	}
	return false
}

func (r *HAProvider) removeMachineIpFromEndpoints(endpoints *corev1.Endpoints, machine *clusterv1.Machine) {
	if endpoints.Subsets == nil || len(endpoints.Subsets) == 0 {
		r.log.Info("currentEndpoints.Subsets is already empty, skip")
		return
	}
	newAddresses := make([]corev1.EndpointAddress, 0)
	for _, address := range endpoints.Subsets[0].Addresses {
		if !r.findEndpointInMachine(address.IP, machine) {
			newAddresses = append(newAddresses, address)
		}
	}
	endpoints.Subsets[0].Addresses = newAddresses
	// remove the Subset if "Addresses" is emtpy
	if len(endpoints.Subsets[0].Addresses) == 0 {
		endpoints.Subsets = nil
	}
}

func (r *HAProvider) addMachineIpToEndpoints(endpoints *corev1.Endpoints, machine *clusterv1.Machine) {
	if endpoints.Subsets == nil {
		// create a Subset if Endpoint doesn't have one
		endpoints.Subsets = []corev1.EndpointSubset{{
			Addresses: make([]corev1.EndpointAddress, 0),
			Ports: []corev1.EndpointPort{{
				Port:     6443,
				Protocol: "TCP",
			}},
		}}
	} else {
		// check if machine has already been added to Endpoints
		for _, address := range endpoints.Subsets[0].Addresses {
			if r.findEndpointInMachine(address.IP, machine) {
				r.log.Info("machine is in Endpoints Object, skip")
				return
			}
		}
	}
	// add a new machine to Endpoints
	for _, machineAddress := range machine.Status.Addresses {
		// check machineAddress.Address is ipv4
		if net.ParseIP(machineAddress.Address) != nil {
			newAddress := corev1.EndpointAddress{
				IP:       machineAddress.Address,
				NodeName: &machine.Name,
			}
			endpoints.Subsets[0].Addresses = append(endpoints.Subsets[0].Addresses, newAddress)
			break
		} else {
			r.log.Info(machineAddress.Address + " is not a valid IP address")
		}
	}
}

func (r *HAProvider) CreateOrUpdateHAEndpoints(ctx context.Context, machine *clusterv1.Machine) error {
	// return if it's not a control plane machine
	if _, ok := machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabelName]; !ok {
		r.log.Info("not a control plane machine, skip")
		return nil
	}

	// get endpoint name (cluster namespace and name)
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      machine.Spec.ClusterName,
		Namespace: machine.Namespace,
	}, cluster); err != nil {
		r.log.Error(err, "Failed to get the cluster of "+machine.Name)
		return err
	}

	endpoints, err := r.ensureEndpoints(ctx, r.getHAServiceName(cluster), cluster.Namespace)
	if err != nil {
		r.log.Error(err, "Failed to get the Endpoints object of current cluster HA Service")
		return err
	}

	if !machine.DeletionTimestamp.IsZero() {
		r.log.Info("machine is being deleted, remove the endpoint of the machine from " + r.getHAServiceName(cluster) + " Endpoints")
		r.removeMachineIpFromEndpoints(endpoints, machine)
	} else {
		// Add machine ip to the Endpoints object no matter it's ready or not
		// Because avi controller checks the status of machine. If it's not ready, avi won't use it as an endpoint
		r.addMachineIpToEndpoints(endpoints, machine)
	}
	return r.Update(ctx, endpoints)
}

func (r *HAProvider) ensureEndpoints(ctx context.Context, serviceName, serviceNamespace string) (*corev1.Endpoints, error) {
	endpoints := &corev1.Endpoints{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      serviceName,
		Namespace: serviceNamespace,
	}, endpoints); err != nil {
		if apierrors.IsNotFound(err) {
			endpoints = &corev1.Endpoints{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Endpoints",
					APIVersion: "core/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: serviceNamespace,
				},
			}
			if err = r.Create(ctx, endpoints); err != nil {
				r.log.Error(err, "Failed to create Endpoints object")
				return nil, err
			}
		} else {
			r.log.Error(err, "Failed to get Endpoints object")
			return nil, err
		}
	}
	return endpoints, nil
}

func GetAviInfraSettingName(adc *akoov1alpha1.AKODeploymentConfig) string {
	return adc.Name + "-ais"
}

func queryFQDNEndpoint(fqdn string) (string, error) {
	ips, err := net.LookupIP(fqdn)
	if err == nil {
		return ips[0].String(), err
	}
	return "", err
}

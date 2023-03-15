// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package cluster_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	akoov1alpha1 "github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/api/v1alpha1"
	"github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/controllers/akodeploymentconfig/cluster"
	"github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/pkg/ako"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const expectedSecretYaml = `#@data/values
#@overlay/match-child-defaults missing_ok=True
---
loadBalancerAndIngressService:
    name: ako--test-cluster
    namespace: avi-system
    config:
        tkg_cluster_role: workload
        is_cluster_service: ""
        replica_count: 1
        ako_settings:
            primary_instance: ""
            log_level: INFO
            full_sync_frequency: "1800"
            api_server_port: 8080
            delete_config: "false"
            disable_static_route_sync: "true"
            cluster_name: -test-cluster
            cni_plugin: ""
            sync_namespace: ""
            enable_EVH: ""
            layer_7_only: ""
            services_api: ""
            vip_per_namespace: ""
            namespace_selector:
                label_key: ""
                label_value: ""
            enable_events: ""
            istio_enabled: ""
            blocked_namespace_list: ""
            ip_family: ""
            use_default_secrets_only: ""
        network_settings:
            subnet_ip: 10.0.0.0
            subnet_prefix: "24"
            network_name: test-akdc
            control_plane_network_name: test-akdc-cp
            control_plane_network_cidr: 10.1.0.0/24
            node_network_list: '[{"networkName":"test-node-network-1","cidrs":["10.0.0.0/24","192.168.0.0/24"]}]'
            vip_network_list: '[{"networkName":"test-akdc","cidr":"10.0.0.0/24"}]'
            enable_rhi: ""
            nsxt_t1_lr: ""
            bgp_peer_labels: ""
        l7_settings:
            disable_ingress_class: true
            default_ing_controller: false
            l7_sharding_scheme: ""
            service_type: NodePort
            shard_vs_size: MEDIUM
            pass_through_shardsize: ""
            no_pg_for_SNI: false
            enable_MCI: ""
        l4_settings:
            default_domain: ""
            auto_fqdn: ""
        controller_settings:
            service_engine_group_name: Default-SEG
            controller_version: 20.1.3
            cloud_name: test-cloud
            controller_ip: 10.23.122.1
            tenant_name: ""
        nodeport_selector:
            key: ""
            value: ""
        rbac:
            psp_enabled: true
            psp_policy_api_version: test/1.2
        persistent_volume_claim: "true"
        mount_path: /var/log
        log_file: test-avi.log
        avi_credentials:
            username: admin
            password: Admin!23
            certificate_authority_data: '-----BEGIN CERTIFICATE-----jf5Hlg==-----END CERTIFICATE-----'
`

func unitTestAKODeploymentYaml() {
	Context("PopulateValues", func() {
		var (
			akoDeploymentConfig *akoov1alpha1.AKODeploymentConfig
			capicluster         *clusterv1.Cluster
			aviUserSecret       *corev1.Secret
		)
		BeforeEach(func() {
			capicluster = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-cluster",
					Labels: map[string]string{},
				},
			}
		})

		When("a valid AKODeploymentYaml is provided", func() {
			BeforeEach(func() {
				akoDeploymentConfig = &akoov1alpha1.AKODeploymentConfig{
					Spec: akoov1alpha1.AKODeploymentConfigSpec{
						CloudName:          "test-cloud",
						Controller:         "10.23.122.1",
						ControllerVersion:  "20.1.3",
						ServiceEngineGroup: "Default-SEG",
						DataNetwork: akoov1alpha1.DataNetwork{
							Name: "test-akdc",
							CIDR: "10.0.0.0/24",
						},
						ControlPlaneNetwork: akoov1alpha1.ControlPlaneNetwork{
							Name: "test-akdc-cp",
							CIDR: "10.1.0.0/24",
						},
						ExtraConfigs: akoov1alpha1.ExtraConfigs{
							Rbac: akoov1alpha1.AKORbacConfig{
								PspEnabled:          pointer.Bool(true),
								PspPolicyAPIVersion: "test/1.2",
							},
							Log: akoov1alpha1.AKOLogConfig{
								PersistentVolumeClaim: "true",
								MountPath:             "/var/log",
								LogFile:               "test-avi.log",
							},
							IngressConfigs: akoov1alpha1.AKOIngressConfig{
								DisableIngressClass:      pointer.Bool(true),
								DefaultIngressController: pointer.Bool(false),
								ShardVSSize:              "MEDIUM",
								ServiceType:              "NodePort",
								NodeNetworkList: []akoov1alpha1.NodeNetwork{
									{
										NetworkName: "test-node-network-1",
										Cidrs:       []string{"10.0.0.0/24", "192.168.0.0/24"},
									},
								},
							},
							DisableStaticRouteSync: pointer.BoolPtr(true),
						},
					},
				}
				aviUserSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-avi-credentials",
						Namespace: "default",
					},
					Type: "Opaque",
					Data: map[string][]byte{
						"username":                 []byte("admin"),
						"password":                 []byte("Admin!23"),
						"certificateAuthorityData": []byte("-----BEGIN CERTIFICATE-----jf5Hlg==-----END CERTIFICATE-----"),
					},
				}
			})

			It("should populate correct values in crs yaml", func() {
				_, err := cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should generate the exact AddonSecretData values", func() {
				secret, err := ako.NewValues(akoDeploymentConfig, "namespace-name")
				Expect(err).ShouldNot(HaveOccurred())
				Expect(secret.LoadBalancerAndIngressService.Name).Should(Equal("ako-namespace-name"))
			})

			It("should generates exact values in crs yaml with the string template approach", func() {
				secretYaml, err := cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(secretYaml).Should(Equal(expectedSecretYaml))
			})

			It("should throw error if template not match", func() {
				akoDeploymentConfig.Spec.DataNetwork.CIDR = "test"
				_, err := cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
				Expect(err).Should(HaveOccurred())
				akoDeploymentConfig.Spec.DataNetwork.CIDR = "10.0.0.0/24"
			})

			It("should update delete_config in this way", func() {
				values, err := ako.NewValues(akoDeploymentConfig, "namespace-name")
				Expect(err).ShouldNot(HaveOccurred())
				values.LoadBalancerAndIngressService.Config.AKOSettings.DeleteConfig = "true"
				secretData, err := values.YttYaml(&clusterv1.Cluster{})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(secretData).Should(ContainSubstring("delete_config: \"true\""))
			})

			When("cluster has avi_delete_config label", func() {
				BeforeEach(func() {
					capicluster.Labels[akoov1alpha1.AviClusterDeleteConfigLabel] = "true"
				})

				It("deleteConfig True in add_on_secret when cluster has avi_delete_config label set to true", func() {
					secretData, err := cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(secretData).Should(ContainSubstring("delete_config: \"true\""))
				})

				It("deleteConfig False in add_on_secret when cluster has avi_delete_config label set to false", func() {
					capicluster.Labels[akoov1alpha1.AviClusterDeleteConfigLabel] = "false"
					secretData, err := cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(secretData).Should(ContainSubstring("delete_config: \"false\""))
				})

				It("deleteConfig True in add_on_secret when cluster has avi_delete_config label set to true", func() {
					secretData, err := cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(secretData).Should(ContainSubstring("delete_config: \"true\""))
				})

				It("deleteConfig False in add_on_secret when cluster has avi_delete_config label set to false", func() {
					delete(capicluster.Labels, akoov1alpha1.AviClusterDeleteConfigLabel)
					secretData, err := cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(secretData).Should(ContainSubstring("delete_config: \"false\""))
				})

				When("management cluster has avi_delete_config label", func() {
					BeforeEach(func() {
						capicluster.Namespace = akoov1alpha1.TKGSystemNamespace
					})

					It("deleteConfig always False in add_on_secret", func() {
						secretData, err := cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(secretData).Should(ContainSubstring("delete_config: \"false\""))

						delete(capicluster.Labels, akoov1alpha1.AviClusterDeleteConfigLabel)
						secretData, err = cluster.AkoAddonSecretDataYaml(capicluster, akoDeploymentConfig, aviUserSecret)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(secretData).Should(ContainSubstring("delete_config: \"false\""))
					})
				})
			})
		})
	})
}

// Copyright 2022 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package default_adc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	akoov1alpha1 "github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/api/v1alpha1"
	"github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/pkg/test/builder"
	testutil "github.com/vmware-tanzu/load-balancer-operator-for-kubernetes/pkg/test/util"
)

func intgTestClusterDisableAVIWithoutAnyADC() {
	var (
		ctx           *builder.IntegrationTestContext
		staticCluster *clusterv1.Cluster
	)

	BeforeEach(func() {
		ctx = suite.NewIntegrationTestContext()
		staticCluster = testutil.GetDefaultCluster()
	})

	When("there is no ADC and a cluster is created", func() {
		BeforeEach(func() {
			testutil.CreateObjects(ctx, staticCluster.DeepCopy())
		})
		AfterEach(func() {
			testutil.DeleteObjects(ctx, staticCluster.DeepCopy())
			testutil.EnsureRuntimeObjectMatchExpectation(ctx, client.ObjectKey{
				Name:      staticCluster.Name,
				Namespace: staticCluster.Namespace,
			}, &clusterv1.Cluster{}, testutil.NOTFOUND)
		})
		It("shouldn't have 'networking.tkg.tanzu.vmware.com/avi'", func() {
			testutil.EnsureClusterAviLabelExists(ctx, client.ObjectKey{
				Name:      staticCluster.Name,
				Namespace: staticCluster.Namespace,
			}, akoov1alpha1.AviClusterLabel, false)
		})
	})
}

func intgTestClusterCanBeSelectedByADC() {
	var (
		ctx *builder.IntegrationTestContext

		staticCluster                    *clusterv1.Cluster
		staticAkoDeploymentConfig        *akoov1alpha1.AKODeploymentConfig
		staticDefaultAkoDeploymentConfig *akoov1alpha1.AKODeploymentConfig

		staticManagementNamespace           *v1.Namespace
		staticManagementCluster             *clusterv1.Cluster
		staticManagementAkoDeploymentConfig *akoov1alpha1.AKODeploymentConfig
	)

	BeforeEach(func() {
		ctx = suite.NewIntegrationTestContext()
		staticCluster = testutil.GetDefaultCluster()
		staticAkoDeploymentConfig = testutil.GetCustomizedADC(testutil.CustomizedADCLabels)
		staticDefaultAkoDeploymentConfig = testutil.GetDefaultADC()

		staticManagementNamespace = &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "tkg-system"},
		}
		staticManagementCluster = testutil.GetManagementCluster()
		staticManagementAkoDeploymentConfig = testutil.GetManagementADC()
	})

	When("both default and customized ADC exist", func() {

		BeforeEach(func() {
			testutil.CreateObjects(ctx, staticAkoDeploymentConfig.DeepCopy())
			testutil.CreateObjects(ctx, staticDefaultAkoDeploymentConfig.DeepCopy())
			testutil.CreateObjects(ctx, staticCluster.DeepCopy())
		})

		It("labels the cluster dynamically", func() {
			By("labels with 'networking.tkg.tanzu.vmware.com/avi: install-ako-for-all'", func() {
				testutil.EnsureClusterAviLabelMatchExpectation(ctx, client.ObjectKey{
					Name:      staticCluster.Name,
					Namespace: staticCluster.Namespace,
				}, akoov1alpha1.AviClusterLabel, staticDefaultAkoDeploymentConfig.Name)
			})

			By("add cluster label to use customized adc")
			testutil.UpdateObjectLabels(ctx, client.ObjectKey{
				Name:      staticCluster.Name,
				Namespace: staticCluster.Namespace,
			}, testutil.CustomizedADCLabels)

			By("labels with 'networking.tkg.tanzu.vmware.com/avi: ako-deployment-config'", func() {
				testutil.EnsureClusterAviLabelMatchExpectation(ctx, client.ObjectKey{
					Name:      staticCluster.Name,
					Namespace: staticCluster.Namespace,
				}, akoov1alpha1.AviClusterLabel, staticAkoDeploymentConfig.Name)
			})

			By("create another customized ako-deployment-config2")
			anotherAkoDeploymentConfig := staticAkoDeploymentConfig.DeepCopy()
			anotherAkoDeploymentConfig.Name = "ako-deployment-config-2"
			testutil.CreateObjects(ctx, anotherAkoDeploymentConfig.DeepCopy())

			By("cluster should keep its label, even through another custom ADC matches the name. a.k.a it won't override", func() {
				Consistently(func() bool {
					obj := &clusterv1.Cluster{}
					err := ctx.Client.Get(ctx.Context, client.ObjectKey{
						Name:      staticCluster.Name,
						Namespace: staticCluster.Namespace,
					}, obj)
					if err != nil {
						return false
					}
					val, ok := obj.Labels[akoov1alpha1.AviClusterLabel]
					return ok && val == staticAkoDeploymentConfig.Name
				})
			})

			By("unset cluster label to use default adc")
			testutil.UpdateObjectLabels(ctx, client.ObjectKey{
				Name:      staticCluster.Name,
				Namespace: staticCluster.Namespace,
			}, map[string]string{})

			By("labels with 'networking.tkg.tanzu.vmware.com/avi: install-ako-for-all'", func() {
				testutil.EnsureClusterAviLabelMatchExpectation(ctx, client.ObjectKey{
					Name:      staticCluster.Name,
					Namespace: staticCluster.Namespace,
				}, akoov1alpha1.AviClusterLabel, staticDefaultAkoDeploymentConfig.Name)
			})
		})
	})

	When("management ADC exists", func() {
		BeforeEach(func() {
			testutil.CreateObjects(ctx, staticManagementNamespace.DeepCopy())
			testutil.CreateObjects(ctx, staticManagementAkoDeploymentConfig.DeepCopy())
			testutil.CreateObjects(ctx, staticManagementCluster.DeepCopy())
		})

		It("labels the management cluster", func() {
			By("labels with 'networking.tkg.tanzu.vmware.com/avi: install-ako-for-management-cluster'", func() {
				testutil.EnsureClusterAviLabelMatchExpectation(ctx, client.ObjectKey{
					Name:      staticManagementCluster.Name,
					Namespace: staticManagementCluster.Namespace,
				}, akoov1alpha1.AviClusterLabel, staticManagementAkoDeploymentConfig.Name)
			})
		})
	})
}

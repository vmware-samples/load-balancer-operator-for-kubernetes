// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package phases

import (
	ako_operator "github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/ako-operator"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	akoov1alpha1 "github.com/vmware-samples/load-balancer-operator-for-kubernetes/api/v1alpha1"
	"github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func ReconcilePhaseUnitTest() {
	var (
		err                 error
		log                 logr.Logger
		ctx                 *builder.IntegrationTestContext
		akoDeploymentConfig *akoov1alpha1.AKODeploymentConfig
	)
	BeforeEach(func() {
		akoDeploymentConfig = &akoov1alpha1.AKODeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ako-deployment-config",
				Namespace: "default",
			},
			Spec: akoov1alpha1.AKODeploymentConfigSpec{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test": "test",
					},
				},
				DataNetwork: akoov1alpha1.DataNetwork{
					Name:    "test",
					CIDR:    "1.1.1.1/20",
					IPPools: []akoov1alpha1.IPPool{},
				},
				ControlPlaneNetwork: akoov1alpha1.ControlPlaneNetwork{
					Name: "integration-test-8ed12g",
					CIDR: "10.1.0.0/24",
				},
				CertificateAuthorityRef: &akoov1alpha1.SecretRef{
					Name:      "test-ca-secret",
					Namespace: "default",
				},
				AdminCredentialRef: &akoov1alpha1.SecretRef{},
			},
		}
		ctx = suite.NewIntegrationTestContext()
		log = ctrl.Log.WithName("controllers").WithName("AKODeploymentConfig")
	})

	Context("Should be able to list all workload clusters", func() {
		var cluster *clusterv1.Cluster

		BeforeEach(func() {
			//ctx = suite.NewIntegrationTestContext()
			cluster = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						akoov1alpha1.AviClusterLabel: "test-ako-deployment-config",
						"test":                       "test",
					},
				},
				Spec: clusterv1.ClusterSpec{},
			}
			err = ctx.Client.Create(ctx.Context, cluster)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err = ctx.Client.Delete(ctx.Context, cluster)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("list all selected workload clusters", func() {
			clusterList, err := ako_operator.ListAkoDeploymentConfigSelectClusters(ctx.Context, ctx.Client, log, akoDeploymentConfig)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(clusterList.Items)).To(Equal(1))
			Expect(clusterList.Items[0].Name).To(Equal("test-cluster"))
		})
	})
}

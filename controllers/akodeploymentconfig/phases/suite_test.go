// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package phases

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/aviclient"
	"github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/test/builder"
	testutil "github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/test/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
)

// suite is used for unit and integration testing this controller.
var suite = builder.NewTestSuiteForController(
	func(mgr ctrlmgr.Manager) error {
		builder.FakeAvi = aviclient.NewFakeAviClient()
		return nil
	},
	func(scheme *runtime.Scheme) (err error) {
		err = corev1.AddToScheme(scheme)
		if err != nil {
			return err
		}
		err = clusterv1.AddToScheme(scheme)
		if err != nil {
			return err
		}
		return nil
	},
	filepath.Join(testutil.FindModuleDir("sigs.k8s.io/cluster-api"), "config", "crd", "bases"),
)

func TestController(t *testing.T) {
	suite.Register(t, "AKO Operator AKODeploymentConfig controller phases reconciler", intgTests, unitTests)
}

var _ = BeforeSuite(suite.BeforeSuite)

var _ = AfterSuite(suite.AfterSuite)

func intgTests() {
	Describe("Phase Reconciler Test", ReconcilePhaseUnitTest)
}

func unitTests() {
}

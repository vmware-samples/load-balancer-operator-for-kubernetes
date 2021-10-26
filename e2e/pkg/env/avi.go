// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/aviclient"

	ako_operator "github.com/vmware-samples/load-balancer-operator-for-kubernetes/pkg/ako-operator"
)

func NewAviRunner(runner *KubectlRunner) aviclient.Client {

	aviClient, _ := aviclient.NewAviClient(&aviclient.AviClientConfig{
		ServerIP: GetAviObject(runner, "akodeploymentconfig", "ako-deployment-config", "spec", "controller"),
		Username: GetAviObject(runner, "secret", "controller-credentials", "data", "username"),
		Password: GetAviObject(runner, "secret", "controller-credentials", "data", "password"),
		CA:       GetAviObject(runner, "secret", "controller-ca", "data", "certificateAuthorityData"),
	}, ako_operator.GetAVIControllerVersion())

	return aviClient
}

func EnsureAviObjectDeleted(aviClient aviclient.Client, clusterName string, obj string) {
	Eventually(func() bool {
		var err error

		switch obj {
		case "virtualservice":
			_, err = aviClient.VirtualServiceGetByName(clusterName + "--default-static-ip")
		case "pool":
			_, err = aviClient.PoolGetByName(clusterName + "--default-static-ip--80")
		default:
			GinkgoT().Logf("EnsureAviObjectDeleted function doesn't support checking " + obj)
			return false
		}

		if err != nil {
			if strings.Contains(err.Error(), "No object of type "+obj) {
				GinkgoT().Logf("No object of type " + obj + " with name " + clusterName + " is found")
				return true
			}
			GinkgoT().Logf("Avi Client query error:" + err.Error())
			return false
		}
		GinkgoT().Logf(obj + " with name " + clusterName + " is found unexpectedly, return false")
		return false
	}, "30s", "5s").Should(BeTrue())
}

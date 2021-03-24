/*
Copyright 2021 The Lynx Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cases

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	groupv1alpha1 "github.com/smartxworks/lynx/pkg/apis/group/v1alpha1"
	securityv1alpha1 "github.com/smartxworks/lynx/pkg/apis/security/v1alpha1"
)

var _ = Describe("SecurityPolicy", func() {

	// This suite
	//
	//        |---------|          |----------- |          |---------- |
	//  --->  |  nginx  |  <---->  | webservers |  <---->  | databases |
	//        | --------|          |----------- |          |---------- |
	//
	Context("environment with virtual machines provide public http service [Feature:TCP] [Feature:ICMP]", func() {
		var nginx, server01, server02, db01, db02 *VM
		var nginxGroup, serverGroup, dbGroup *groupv1alpha1.EndpointGroup

		BeforeEach(func() {
			nginx = &VM{Name: "nginx", TCPPort: 443, Labels: "component=nginx"}
			db01 = &VM{Name: "db01", TCPPort: 3306, Labels: "component=database"}
			db02 = &VM{Name: "db02", TCPPort: 3306, Labels: "component=database"}
			server01 = &VM{Name: "server01", TCPPort: 443, Labels: "component=webserver"}
			server02 = &VM{Name: "server02", TCPPort: 443, Labels: "component=webserver"}

			nginxGroup = newGroup("nginx", "component=nginx")
			serverGroup = newGroup("database", "component=database")
			dbGroup = newGroup("webserver", "component=webserver")

			Expect(e2eEnv.SetupVMs(nginx, server01, server02, db01, db02)).Should(Succeed())
			Expect(e2eEnv.SetupObjects(nginxGroup, serverGroup, dbGroup)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(e2eEnv.CleanVMs(nginx, server01, server02, db01, db02)).Should(Succeed())
			Expect(e2eEnv.CleanObjects(nginxGroup, serverGroup, dbGroup)).Should(Succeed())
		})

		When("limits tcp packets between components", func() {
			var nginxPolicy, serverPolicy, dbPolicy *securityv1alpha1.SecurityPolicy

			BeforeEach(func() {
				// todo: new securitypolicy
				// allow icmp to nginx
				Expect(e2eEnv.SetupObjects(nginxPolicy, serverPolicy, dbPolicy)).Should(Succeed())
			})

			AfterEach(func() {
				Expect(e2eEnv.CleanObjects(nginxPolicy, serverPolicy, dbPolicy)).Should(Succeed())
			})

			It("should allow normal packets", func() {
				assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICP", true)
				assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICMP", true)

				assertReachable([]*VM{nginx}, []*VM{server01, server02}, "TCP", true)
				assertReachable([]*VM{server01, server02}, []*VM{db01, db02}, "TCP", true)
			})

			It("should limits illegal packets", func() {
				assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICP", true)
				assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICMP", true)

				assertReachable([]*VM{nginx}, []*VM{server01, server02}, "TCP", true)
				assertReachable([]*VM{server01, server02}, []*VM{db01, db02}, "TCP", true)
			})

			When("add virtual machine into the database group", func() {
				var db03 *VM

				BeforeEach(func() {
					db03 = &VM{Name: "db03", TCPPort: 3306, Labels: "component=database"}
					Expect(e2eEnv.SetupVMs(db03)).Should(Succeed())
				})

				AfterEach(func() {
					Expect(e2eEnv.CleanVMs(db03)).Should(Succeed())
				})

				It("should allow normal packets", func() {
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICP", true)
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICMP", true)

					assertReachable([]*VM{nginx}, []*VM{server01, server02}, "TCP", true)
					assertReachable([]*VM{server01, server02}, []*VM{db01, db02}, "TCP", true)
				})

				It("should limits illegal packets", func() {
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICP", true)
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICMP", true)

					assertReachable([]*VM{nginx}, []*VM{server01, server02}, "TCP", true)
					assertReachable([]*VM{server01, server02}, []*VM{db01, db02}, "TCP", true)
				})
			})

			When("update virtual machine ip address in the nginx group", func() {
				BeforeEach(func() {
					Expect(e2eEnv.UpdateVMRandIP(nginx)).Should(Succeed())
				})

				It("should allow normal packets for new group member", func() {
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICP", true)
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICMP", true)

					assertReachable([]*VM{nginx}, []*VM{server01, server02}, "TCP", true)
					assertReachable([]*VM{server01, server02}, []*VM{db01, db02}, "TCP", true)
				})

				It("should limits illegal packets for new group member", func() {
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICP", true)
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICMP", true)

					assertReachable([]*VM{nginx}, []*VM{server01, server02}, "TCP", true)
					assertReachable([]*VM{server01, server02}, []*VM{db01, db02}, "TCP", true)
				})
			})

			When("remove virtual machine from the webserver group", func() {
				BeforeEach(func() {
					// remove webserver02 labels to remove it from webserver group
					server02.Labels = ""
					Expect(e2eEnv.UpdateVMLabels(server02)).Should(Succeed())
				})

				It("should limits illegal packets for the remove group member", func() {
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICP", true)
					assertReachable([]*VM{server01, server02, db01, db02}, []*VM{nginx}, "ICMP", true)

					assertReachable([]*VM{nginx}, []*VM{server01, server02}, "TCP", true)
					assertReachable([]*VM{server01, server02}, []*VM{db01, db02}, "TCP", true)
				})
			})
		})

		When("limits icmp packets between components", func() {
			It("should allow normal icmp packets", func() {
			})

			It("should limits illegal icmp packets", func() {
			})
		})

		// todo: isolate virtual machine
		XWhen("isolate virtual machine with viruses", func() {})
	})

	// 指定特定 cidr 虚拟只能访问 cidr 范围内的 ntp 服务器，两个 ntp 服务器之间可以互相访问
	// This suite
	//
	//  |----------------|         |--------------- |         |---------------- |         |--------------- |
	//  | 192.168.1.0/24 |  <--->  | ntp-production |  <--->  | ntp-development |  <--->  | 192.168.2.0/24 |
	//  | ---------------|         |--------------- |         |---------------- |         |--------------- |
	//
	Context("environment with virtual machines provide internal udp service [Feature:UDP] [Feature:IPBlocks]", func() {
		var ntp01, ntp02, client01, client02 *VM
		var ntpProduction, ntpDevelopment *groupv1alpha1.EndpointGroup

		BeforeEach(func() {
			client01 = &VM{Name: "ntp-client01", ExpectCidr: "10.0.0.0/28"}
			client02 = &VM{Name: "ntp-client02", ExpectCidr: "10.0.0.16/28"}
			ntp01 = &VM{Name: "ntp01-server", ExpectCidr: "10.0.0.0/28", UDPPort: 123, Labels: "component=ntp,env=production"}
			ntp02 = &VM{Name: "ntp02-server", ExpectCidr: "10.0.0.16/28", UDPPort: 123, Labels: "component=ntp,env=development"}

			ntpProduction = newGroup("ntp-production", "component=ntp,env=production")
			ntpDevelopment = newGroup("ntp-development", "component=ntp,env=development")

			Expect(e2eEnv.SetupVMs(ntp01, ntp02, client01, client02)).Should(Succeed())
			Expect(e2eEnv.SetupObjects(ntpProduction, ntpDevelopment)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(e2eEnv.CleanVMs(ntp01, ntp02, client01, client02)).Should(Succeed())
			Expect(e2eEnv.CleanObjects(ntpProduction)).Should(Succeed())
		})

		When("limits udp packets by ipBlocks between server and client", func() {
			var ntpProductionPolicy, ntpDevelopmentPolicy *securityv1alpha1.SecurityPolicy

			BeforeEach(func() {
				ntpProductionPolicy = newPolicy("ntp-own-cidr-allow-only")
				ntpDevelopmentPolicy = newPolicy()
			})

			AfterEach(func() {
				Expect(e2eEnv.CleanObjects(ntpProductionPolicy, ntpDevelopmentPolicy)).Should(Succeed())
			})

			It("should allow normal udp packets", func() {
				By("verify reachable between servers")
				assertReachable([]*VM{ntp01}, []*VM{ntp02}, "UDP", true)
				assertReachable([]*VM{ntp02}, []*VM{ntp01}, "UDP", true)

				By("verify reachable between server and client")
				assertReachable([]*VM{client01}, []*VM{ntp01}, "UDP", true)
				assertReachable([]*VM{client02}, []*VM{ntp02}, "UDP", true)
			})

			It("should limits illegal udp packets", func() {
				assertReachable([]*VM{client01}, []*VM{ntp02}, "UDP", false)
				assertReachable([]*VM{client02}, []*VM{ntp01}, "UDP", false)
			})
		})
	})
})

func newGroup(name string, labels string) *groupv1alpha1.EndpointGroup {
	group := &groupv1alpha1.EndpointGroup{}
	group.Name = name
	selector := asMapLables(labels)

	group.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: selector,
	}

	return group
}

// todo:
func newPolicy(arguments ...interface{}) *securityv1alpha1.SecurityPolicy {
	panic(arguments)
}

func assertReachable(froms []*VM, tos []*VM, protocol string, reachable bool) {
	Eventually(func() error {
		for _, from := range froms {
			for _, to := range tos {
				if reachable != e2eEnv.Reachable(from, to, protocol) {
					return fmt.Errorf("get reachable %t, want %t. from: %+v, to: %+v, protocol: %s", reachable, !reachable, from, to, protocol)
				}
			}
		}
		return nil
	}, e2eEnv.Timeout(), e2eEnv.Interval()).Should(Succeed())
}

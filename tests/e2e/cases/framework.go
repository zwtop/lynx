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
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	agentv1alpha1 "github.com/smartxworks/lynx/pkg/apis/agent/v1alpha1"
	groupv1alpha1 "github.com/smartxworks/lynx/pkg/apis/group/v1alpha1"
	policyv1alpha1 "github.com/smartxworks/lynx/pkg/apis/policyrule/v1alpha1"
	securityv1alpha1 "github.com/smartxworks/lynx/pkg/apis/security/v1alpha1"
)

type Framework struct {
	ctx       context.Context
	k8sClient client.Client

	ipPoolLock sync.RWMutex
	ipPoolCidr string
	ipUsed     map[string]bool // list of ips has been assigned

	agentsLock sync.RWMutex // agents read/write lock
	agents     []string     // agents name or ip address

	timeout  time.Duration
	interval time.Duration
}

func FrameworkFromConfig(configFile string) (*Framework, error) {
	var err error

	var e2eEnv = &Framework{
		ctx:        context.Background(),
		ipPoolCidr: "10.0.0.0/24",
		ipUsed:     make(map[string]bool),
		timeout:    time.Second * 10,
		interval:   time.Millisecond * 250,
	}

	e2eEnv.k8sClient, err = client.New(config.GetConfigOrDie(), client.Options{
		Scheme: addLynxToScheme(runtime.NewScheme()),
	})
	if err != nil {
		return nil, err
	}

	agents, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	e2eEnv.agentsLock.Lock()
	defer e2eEnv.agentsLock.Unlock()

	for _, agent := range strings.Split(string(agents), ",") {
		e2eEnv.agents = append(e2eEnv.agents, agent)
	}

	if len(e2eEnv.agents) == 0 {
		return nil, fmt.Errorf("at least one agent should provided")
	}

	return e2eEnv, nil
}

type VM struct {
	Name   string
	Labels string

	ExpectCidr string
	TCPPort    int32
	UDPPort    int32

	status *vmStatus
}

type vmStatus struct {
	ipAddr string
	agent  string
	netns  string
}

func (e *Framework) SetupVMs(vms ...*VM) error {
	for _, vm := range vms {
		if vm.status != nil {
			return fmt.Errorf("vm %s has been setup already", vm.Name)
		}

		var vmNetns = fmt.Sprintf("e2e-%s", rand.String(5))
		var vmEp = toEndpoint(vmNetns, vm.Labels)
		var err error

		vm.status = &vmStatus{
			agent: e.randomAgent(),
			netns: vmNetns,
		}

		if vm.ExpectCidr == "" {
			vm.ExpectCidr = e.ipPoolCidr
		}
		vm.status.ipAddr, err = e.randomIPv4(vm.ExpectCidr)
		if err != nil {
			return fmt.Errorf("get random ip for vm %s: %s", vm.Name, err)
		}

		stdout, rc, err := runScriptRemote(vm.status.agent, startNewVM, vm.status.netns, vm.status.ipAddr, string(vm.TCPPort), string(vm.UDPPort))
		if err != nil {
			return err
		}
		if rc != 0 {
			return fmt.Errorf(string(stdout))
		}

		if err := e.k8sClient.Create(e.ctx, vmEp); err != nil {
			return err
		}
	}
	return nil
}

func (e *Framework) CleanVMs(vms ...*VM) error {
	for _, vm := range vms {
		if vm.status == nil {
			return fmt.Errorf("cant clean vm %s because of vm haven't setup yet", vm.Name)
		}

		stdout, rc, err := runScriptRemote(vm.status.agent, destroyVM, vm.status.netns)
		if err != nil {
			return err
		}
		if rc != 0 {
			return fmt.Errorf(string(stdout))
		}

		var vmEp = toEndpoint(vm.status.netns, vm.Labels)
		if err := e.k8sClient.Delete(e.ctx, vmEp); err != nil {
			return err
		}
	}
	return nil
}

// todo:
func (e *Framework) UpdateVMLabels(vm *VM) error {
	return nil
}

func (e *Framework) UpdateVMRandIP(vm *VM) error {
	var expectIPv4, err = e.randomIPv4(vm.ExpectCidr)
	if err != nil {
		return fmt.Errorf("get random ip for vm %s: %s", vm.Name, err)
	}

	stdout, rc, err := runScriptRemote(vm.status.agent, updateVMIP, vm.status.netns, expectIPv4)
	if err != nil {
		return err
	}
	if rc != 0 {
		return fmt.Errorf(string(stdout))
	}

	vm.status.ipAddr = expectIPv4
	return nil
}

func (e *Framework) SetupObjects(objects ...metav1.Object) error {
	for _, object := range objects {
		err := e.k8sClient.Create(e.ctx, object.(runtime.Object).DeepCopyObject())

		if errors.IsAlreadyExists(err) {
			continue
		}

		if err != nil {
			return fmt.Errorf("unable create object %s: %s", object.GetName(), err)
		}
	}

	return nil
}

func (e *Framework) CleanObjects(objects ...metav1.Object) error {
	for _, object := range objects {
		for {
			// try delete until object removed from kubernetes
			err := e.k8sClient.Delete(e.ctx, object.(runtime.Object).DeepCopyObject())
			if errors.IsNotFound(err) {
				break
			}
		}
	}

	return nil
}

func (e *Framework) Reachable(from *VM, to *VM, protocol string) bool {
	switch protocol {
	case "TCP":
		_, rc, err := runScriptRemote(from.status.agent, tcpReachable, to.status.ipAddr, string(to.TCPPort))
		return rc == 0 && err == nil
	case "UDP":
		_, rc, err := runScriptRemote(from.status.agent, udpReachable, to.status.ipAddr, string(to.UDPPort))
		return rc == 0 && err == nil
	case "ICMP":
		_, rc, err := runScriptRemote(from.status.agent, icmpReachable, to.status.ipAddr)
		return rc == 0 && err == nil
	}

	return false
}

func (e *Framework) Timeout() time.Duration {
	return e.timeout
}

func (e *Framework) Interval() time.Duration {
	return e.interval
}

func (e *Framework) randomAgent() string {
	e.agentsLock.RLock()
	defer e.agentsLock.RUnlock()

	return e.agents[rand.Intn(len(e.agents))]
}

func (e *Framework) randomIPv4(cidr string) (string, error) {
	e.ipPoolLock.Lock()
	defer e.ipPoolLock.Unlock()

	if !containsCidr(e.ipPoolCidr, cidr) {
		return "", fmt.Errorf("cidr %s not in ip pool %s", cidr, e.ipPoolCidr)
	}

	for {
		var randomIPv4 = randomIPv4FromCidr(cidr)

		if _, ok := e.ipUsed[randomIPv4]; !ok {
			e.ipUsed[randomIPv4] = true
			return randomIPv4, nil
		}
	}
}

func asMapLables(labels string) map[string]string {
	var labelList = strings.Split(labels, ",")
	var mapLabels = make(map[string]string, len(labelList))

	for _, label := range labelList {
		mapLabels[strings.Split(label, "=")[0]] = strings.Split(label, "=")[1]
	}

	return mapLabels
}

func randomIPv4FromCidr(cidr string) string {
	var _, netCidr, _ = net.ParseCIDR(cidr)
	var maskSize, _ = netCidr.Mask.Size()
	var offset = rand.Intn(1 << (32 - maskSize))

	var ipToI32 = func(ip net.IP) int32 {
		ip = ip.To4()
		return int32(ip[0])<<24 | int32(ip[1])<<16 | int32(ip[2])<<8 | int32(ip[3])
	}

	var i32ToIP = func(a int32) net.IP {
		return net.IPv4(byte(a>>24), byte(a>>16), byte(a>>8), byte(a))
	}

	return i32ToIP(ipToI32(netCidr.IP) + int32(offset)).String()
}

func containsCidr(cidr1, cidr2 string) bool {
	var _, netCidr1, _ = net.ParseCIDR(cidr1)
	var _, netCidr2, _ = net.ParseCIDR(cidr2)

	var maskSize1, _ = netCidr1.Mask.Size()
	var maskSize2, _ = netCidr2.Mask.Size()

	if maskSize1 > maskSize2 {
		return false
	}

	return netCidr1.Contains(netCidr1.IP)
}

func toEndpoint(name string, lables string) *securityv1alpha1.Endpoint {
	return &securityv1alpha1.Endpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: asMapLables(lables),
		},
		Spec: securityv1alpha1.EndpointReference{
			ExternalIDName:  "external_uuid",
			ExternalIDValue: "uuid-" + name,
		},
	}
}

func addLynxToScheme(scheme *runtime.Scheme) *runtime.Scheme {
	_ = policyv1alpha1.AddToScheme(scheme)
	_ = securityv1alpha1.AddToScheme(scheme)
	_ = groupv1alpha1.AddToScheme(scheme)
	_ = agentv1alpha1.AddToScheme(scheme)

	return scheme
}

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

package endpoint

import (
	"context"
	"fmt"
	"k8s.io/klog"
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/util/errors"

	"github.com/smartxworks/lynx/pkg/client/clientset_generated/clientset"
	"github.com/smartxworks/lynx/tests/e2e/framework/config"
	"github.com/smartxworks/lynx/tests/e2e/framework/endpoint/netns"
	"github.com/smartxworks/lynx/tests/e2e/framework/endpoint/tower"
	"github.com/smartxworks/lynx/tests/e2e/framework/ipam"
	"github.com/smartxworks/lynx/tests/e2e/framework/model"
	"github.com/smartxworks/lynx/tests/e2e/framework/node"
)

type Manager struct {
	model.EndpointProvider
}

func NewManager(pool ipam.Pool, nodeManager *node.Manager, config *config.EndpointConfig) *Manager {
	var provider model.EndpointProvider

	switch {
	case config.Provider == nil, *config.Provider == "netns":
		crdClient := clientset.NewForConfigOrDie(config.KubeConfig)
		provider = netns.NewProvider(pool, nodeManager, crdClient)
	case *config.Provider == "tower":
		provider = tower.NewProvider(pool, nodeManager, config.TowerClient, *config.VMTemplateID)
	default:
		panic("unknown provider " + *config.Provider)
	}

	return &Manager{EndpointProvider: provider}
}

func (m *Manager) SetupMany(ctx context.Context, endpoints ...*model.Endpoint) error {
	var errList []error
	for _, endpoint := range endpoints {
		if _, err := m.Create(ctx, endpoint); err != nil {
			errList = append(errList, err)
		}
	}
	return errors.NewAggregate(errList)
}

func (m *Manager) CleanMany(ctx context.Context, endpoints ...*model.Endpoint) error {
	var errList []error
	for _, endpoint := range endpoints {
		if err := m.Delete(ctx, endpoint.Name); err != nil {
			errList = append(errList, err)
		}
	}
	return errors.NewAggregate(errList)
}

func (m *Manager) UpdateMany(ctx context.Context, endpoints ...*model.Endpoint) error {
	var errList []error
	for _, endpoint := range endpoints {
		if _, err := m.Update(ctx, endpoint); err != nil {
			errList = append(errList, err)
		}
	}
	return errors.NewAggregate(errList)
}

func (m *Manager) MigrateMany(ctx context.Context, endpoints ...*model.Endpoint) error {
	var errList []error
	for _, endpoint := range endpoints {
		ep, err := m.Migrate(ctx, endpoint.Name)
		if err != nil {
			errList = append(errList, err)
		}
		// update request endpoint status
		endpoint.Status = ep.Status
	}
	return errors.NewAggregate(errList)
}

func (m *Manager) RenewIPMany(ctx context.Context, endpoints ...*model.Endpoint) error {
	var errList []error
	for _, endpoint := range endpoints {
		ep, err := m.RenewIP(ctx, endpoint.Name)
		if err != nil {
			errList = append(errList, err)
		}
		// update request endpoint status
		endpoint.Status = ep.Status
	}
	return errors.NewAggregate(errList)
}

func (m *Manager) Reachable(ctx context.Context, src string, dst string, protocol string, port int) (bool, error) {
	var cmd = `web-utils`
	var args = []string{}

	dstEp, err := m.Get(ctx, dst)
	if err != nil {
		return false, fmt.Errorf("unable get dest endpoint: %s", err)
	}

	ip, _, err := net.ParseCIDR(dstEp.Status.IPAddr)
	if err != nil {
		return false, fmt.Errorf("unexpect ipaddr %s of %s", dstEp.Status.IPAddr, dstEp.Name)
	}

	switch strings.ToUpper(protocol) {
	case "TCP", "UDP":
		args = []string{`connect`, `--protocol`, protocol, `--timeout`, "1s", `--server`, fmt.Sprintf("%s:%d", ip, port)}
	case "ICMP":
		args = []string{`connect`, `--protocol`, protocol, `--timeout`, "1s", `--server`, ip.String()}
	default:
		return false, fmt.Errorf("unknow protocol %s", protocol)
	}

	rc, out, err := m.RunCommand(ctx, src, cmd, args...)
	klog.Infof("connect from %s to %s, command: web-utils %s, result: %s", src, dst, strings.Join(args, " "), string(out))

	return rc == 0, err
}

func (m *Manager) ReachTruthTable(ctx context.Context, protocol string, port int) (*model.TruthTable, error) {
	var epList []string
	var errList []error

	eps, err := m.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, ep := range eps {
		epList = append(epList, ep.Name)
	}

	tt := model.NewTruthTableFromItems(epList, nil)
	// fixme: use goroutine. The concurrency depends on the number of sessions configured by sshd
	for _, src := range epList {
		for _, dst := range epList {
			reach, err := m.Reachable(ctx, src, dst, protocol, port)
			tt.Set(src, dst, reach)
			errList = append(errList, err)
		}
	}

	return tt, errors.NewAggregate(errList)
}

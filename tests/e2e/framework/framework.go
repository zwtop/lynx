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

package framework

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/smartxworks/lynx/pkg/client/clientset_generated/clientset/scheme"
	"github.com/smartxworks/lynx/tests/e2e/framework/config"
	"github.com/smartxworks/lynx/tests/e2e/framework/endpoint"
	"github.com/smartxworks/lynx/tests/e2e/framework/ipam"
	"github.com/smartxworks/lynx/tests/e2e/framework/node"
)

type Framework struct {
	kubeClient client.Client

	epManager   *endpoint.Manager
	nodeManager *node.Manager

	timeout  time.Duration
	interval time.Duration
}

func NewFromKube(kubeConfig string) (*Framework, error) {
	var err error
	var cfg *config.Config

	if cfg, err = config.LoadDefault(kubeConfig); err != nil {
		return nil, fmt.Errorf("unable load config from %s: %s", kubeConfig, err)
	}

	var ipPool ipam.Pool
	if ipPool, err = ipam.NewPool(cfg.IPAM); err != nil {
		return nil, fmt.Errorf("unable get ippool: %s", err)
	}

	var kubeClient client.Client
	if kubeClient, err = client.New(cfg.KubeConfig, client.Options{Scheme: scheme.Scheme}); err != nil {
		return nil, fmt.Errorf("unable get kube client: %s", err)
	}

	var nodeManager *node.Manager
	if nodeManager, err = node.NewManagerFromConfig(cfg.Nodes); err != nil {
		return nil, fmt.Errorf("unable get node manager: %s", err)
	}

	f := &Framework{
		kubeClient:  kubeClient,
		epManager:   endpoint.NewManager(ipPool, nodeManager, &cfg.Endpoint),
		nodeManager: nodeManager,
		timeout:     *cfg.Timeout,
		interval:    *cfg.Interval,
	}

	return f, nil
}

func (f *Framework) NodeManager() *node.Manager {
	return f.nodeManager
}

func (f *Framework) EndpointManager() *endpoint.Manager {
	return f.epManager
}

func (f *Framework) KubeClient() client.Client {
	return f.kubeClient
}

func (f *Framework) SetupObjects(ctx context.Context, objects ...metav1.Object) error {
	for _, object := range objects {
		err := f.kubeClient.Create(ctx, object.(runtime.Object).DeepCopyObject())
		if err != nil {
			return fmt.Errorf("unable create object %s: %s", object.GetName(), err)
		}

		err = wait.Poll(f.Interval(), f.Timeout(), func() (done bool, err error) {
			var objKey = types.NamespacedName{Name: object.GetName()}
			var obj = object.(runtime.Object)
			var getErr = f.kubeClient.Get(ctx, objKey, obj.DeepCopyObject())
			return getErr == nil, nil
		})
		if err != nil {
			return fmt.Errorf("unable wait for object %s create: %s", object.GetName(), err)
		}
	}
	return nil
}

func (f *Framework) CleanObjects(ctx context.Context, objects ...metav1.Object) error {
	for _, object := range objects {
		err := f.kubeClient.Delete(ctx, object.(runtime.Object).DeepCopyObject())
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("unable remove object %s: %s", object.GetName(), err)
		}

		err = wait.Poll(f.Interval(), f.Timeout(), func() (done bool, err error) {
			var objKey = types.NamespacedName{Name: object.GetName()}
			var obj = object.(runtime.Object)
			var getErr = f.kubeClient.Get(ctx, objKey, obj.DeepCopyObject())
			return errors.IsNotFound(getErr), nil
		})
		if err != nil {
			return fmt.Errorf("unable wait for object %s delete: %s", object.GetName(), err)
		}
	}

	return nil
}

func (f *Framework) Timeout() time.Duration {
	return f.timeout
}

func (f *Framework) Interval() time.Duration {
	return f.interval
}

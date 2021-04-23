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

package controller

import (
	"context"
	"fmt"
	"github.com/smartxworks/lynx/pkg/client/informers_generated/externalversions"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/smartxworks/lynx/pkg/client/clientset_generated/clientset"
	"github.com/smartxworks/lynx/plugin/tower/pkg/informer"
	"github.com/smartxworks/lynx/plugin/tower/pkg/schema"
)

type controller struct {
	// name of this controller
	name string

	client *clientset.Clientset

	vmInformer       cache.SharedIndexInformer
	vmLister         informer.Lister
	vmInformerSynced cache.InformerSynced

	labelInformer       cache.SharedIndexInformer
	labelLister         informer.Lister
	labelInformerSynced cache.InformerSynced

	endpointInformer       cache.SharedIndexInformer
	endpointLister         informer.Lister
	endpointInformerSynced cache.InformerSynced

	endpointQueue workqueue.RateLimitingInterface
}

const (
	vnicIndex = "vnicIndex"
	vmIndex   = "vmIndex"
)

// New creates a new instance of controller.
func New(towerFactory informer.SharedInformerFactory, crdFactory externalversions.SharedInformerFactory, kubeClient rest.Interface) *controller {
	resyncPeriod := 0 * time.Second

	vmInformer := towerFactory.VM()
	labelInformer := towerFactory.Label()
	endpointInforer := crdFactory.Security().V1alpha1().Endpoints().Informer()

	c := &controller{
		name:                   "EndpointController",
		client:                 clientset.New(kubeClient),
		vmInformer:             vmInformer,
		vmLister:               vmInformer.GetIndexer(),
		vmInformerSynced:       vmInformer.HasSynced,
		labelInformer:          labelInformer,
		labelLister:            labelInformer.GetIndexer(),
		labelInformerSynced:    labelInformer.HasSynced,
		endpointInformer:       endpointInforer,
		endpointLister:         endpointInforer.GetIndexer(),
		endpointInformerSynced: endpointInforer.HasSynced,
		endpointQueue:          workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	// ignore error, error only when informer has already started
	_ = vmInformer.AddIndexers(
		cache.Indexers{
			vnicIndex: func(obj interface{}) ([]string, error) {
				var vnics []string
				for _, vnic := range obj.(*schema.VM).Vnics {
					vnics = append(vnics, vnic.GetID())
				}
				return vnics, nil
			},
		},
	)

	_ = labelInformer.AddIndexers(
		cache.Indexers{
			vmIndex: func(obj interface{}) ([]string, error) {
				var vms []string
				for _, vm := range obj.(*schema.Label).VMs {
					vms = append(vms, vm.ID)
				}
				return vms, nil
			},
		},
	)

	vmInformer.AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addVM,
			UpdateFunc: c.updateVM,
			DeleteFunc: c.deleteVM,
		},
		resyncPeriod,
	)

	labelInformer.AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addLabel,
			UpdateFunc: c.updateLabel,
			DeleteFunc: c.deleteLabel,
		},
		resyncPeriod,
	)

	endpointInforer.AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addEndpoint,
			UpdateFunc: c.updateEndpoint,
			DeleteFunc: c.deleteEndpoint,
		},
		resyncPeriod,
	)

	return c
}

// Run begins processing items, and will continue until a value is sent down stopCh or it is closed.
func (c *controller) Run(works int, stopCh <-chan struct{}) {
	defer c.endpointQueue.ShutDown()

	if !cache.WaitForNamedCacheSync(c.name, stopCh, c.vmInformerSynced, c.labelInformerSynced, c.endpointInformerSynced) {
		return
	}

	for i := 0; i < works; i++ {
		go wait.Until(c.syncEndpointWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *controller) addVM(new interface{}) {

}

func (c *controller) updateVM(old interface{}, new interface{}) {

}

func (c *controller) deleteVM(old interface{}) {

}

func (c *controller) addLabel(new interface{}) {

}

func (c *controller) updateLabel(old interface{}, new interface{}) {

}

func (c *controller) deleteLabel(old interface{}) {

}

func (c *controller) addEndpoint(new interface{}) {

}

func (c *controller) updateEndpoint(old interface{}, new interface{}) {

}

func (c *controller) deleteEndpoint(old interface{}) {

}

func (c *controller) syncEndpointWorker() {
	for {
		key, quit := c.endpointQueue.Get()
		if quit {
			return
		}

		err := c.syncEndpoint(key.(string))
		if err != nil {
			c.endpointQueue.AddRateLimited(key)
			klog.Errorf("got error while sync endpoint %s: %s", key.(string), err)
			continue
		}

		c.endpointQueue.Forget(key)
	}
}

func (c *controller) syncEndpoint(key string) error {
	vms, err := c.vmLister.IndexKeys(vnicIndex, key)
	if err != nil {
		return err
	}

	switch len(vms) {
	case 0:
		// delete this endpoint
		return c.processEndpointDelete(key)
	case 1:
		// create or update endpoint
		return c.processEndpointUpdate(vms[0], key)
	default:
		return fmt.Errorf("got multiple vms %+v for vnic %s", vms, key)
	}
}

func (c *controller) processEndpointDelete(key string) error {
	err := c.client.SecurityV1alpha1().Endpoints().Delete(context.Background(), key, metav1.DeleteOptions{})
	if err == nil || errors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *controller) processEndpointUpdate(vmKey, vnicKey string) error {
	labels, err := c.labelLister.ByIndex(vmIndex, vmKey)
	if err != nil {
		return err
	}

	var vnicLabels = make(map[string]string)
	for _, label := range labels {
		vnicLabels[label.(*schema.Label).Key] = label.(*schema.Label).Value
	}

	if !exists {
		c.client.SecurityV1alpha1().Endpoints().Create(context.Background())
	}

	if !reflect.DeepEqual(endpoint.Labels, vnicLabels) {

	}

	return nil
}

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

package informer

import (
	"fmt"
	"github.com/smartxworks/lynx/plugin/tower/pkg/client"
	"k8s.io/client-go/tools/cache"
	"testing"
)

func TestReflector(t *testing.T) {
	var c = &client.Client{Url: "ws://tower.smartx.com:8800"}

	f := NewSharedInformerFactory(c, 0)
	f.VM()
	f.Label().AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			fmt.Println("create", obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			fmt.Println("update", oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			fmt.Println("delete", obj)
		},
	})
	f.Start(make(chan struct{}))

	select {}
}

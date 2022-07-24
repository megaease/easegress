/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ingresscontroller

import (
	"fmt"
	"os"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/supervisor"
)

type (
	// IngressController is the ingress controller.
	IngressController struct {
		mutex sync.RWMutex

		superSpec *supervisor.Spec
		spec      *spec.Admin

		informer   informer.Informer
		service    *service.Service
		tc         *trafficcontroller.TrafficController
		instanceID string
		IP         string
		namespace  string

		httpServer *supervisor.ObjectEntity
		// key is the backend name instead of pipeline name.
		backendPipelines map[string]*supervisor.ObjectEntity
		ingressBackends  map[string]struct{}
		ingressRules     []*spec.IngressRule
	}

	// Status is the traffic controller status
	Status = trafficcontroller.NamespacesStatus
)

// New creates a mesh ingress controller.
func New(superSpec *supervisor.Spec) *IngressController {
	entity, exists := superSpec.Super().GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}

	store := storage.New(superSpec.Name(), superSpec.Super().Cluster())

	instanceID := os.Getenv(spec.PodEnvHostname)
	applicationIP := os.Getenv(spec.PodEnvApplicationIP)

	if len(instanceID) == 0 || len(applicationIP) == 0 {
		panic(fmt.Errorf("Need environment HOSTNAME: %s and APPLICATIONIP: %sto start ingress controller", instanceID, applicationIP))
	}

	ic := &IngressController{
		superSpec: superSpec,
		spec:      superSpec.ObjectSpec().(*spec.Admin),

		informer:  informer.NewInformer(store, ""),
		service:   service.New(superSpec),
		tc:        tc,
		namespace: superSpec.Name(),

		backendPipelines: make(map[string]*supervisor.ObjectEntity),
		ingressBackends:  make(map[string]struct{}),
		ingressRules:     []*spec.IngressRule{},
		instanceID:       instanceID,
		IP:               applicationIP,
	}

	ic.putIngressControllerInstance()

	err := ic.informer.OnAllIngressSpecs(ic.handleIngresses)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("watch ingress failed: %v", err)
	}

	err = ic.informer.OnAllServiceSpecs(ic.handleServices)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("watch service failed: %v", err)
	}

	err = ic.informer.OnAllServiceInstanceSpecs(ic.handleServiceInstances)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("watch service instance failed: %v", err)
	}

	// using informer for watching ingress cert
	err = ic.informer.OnIngressControllerCert(ic.instanceID, ic.handleCert)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("watch ingress controller cert failed: %v", err)
	}

	return ic
}

func (ic *IngressController) putIngressControllerInstance() {
	instance := &spec.ServiceInstanceSpec{
		RegistryName: "",
		ServiceName:  spec.IngressControllerName,
		InstanceID:   ic.instanceID,
		IP:           ic.IP,
		Status:       spec.ServiceStatusUp,
	}
	ic.service.PutIngressControllerInstanceSpec(instance)
}

func (ic *IngressController) handleIngresses(ingresses map[string]*spec.Ingress) (continueWatch bool) {
	continueWatch = true

	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: handleIngress recover from: %v, stack trace:\n%s\n",
				ic.superSpec.Name(), err, debug.Stack())
		}
	}()

	ic.reloadTraffic()

	return
}

func (ic *IngressController) handleCert(event informer.Event, cert *spec.Certificate) (continueWatch bool) {
	continueWatch = true

	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: handleIngress recover from: %v, stack trace:\n%s\n",
				ic.superSpec.Name(), err, debug.Stack())
		}
	}()

	ic.reloadTraffic()
	return
}

func (ic *IngressController) handleServices(services map[string]*spec.Service) (continueWatch bool) {
	continueWatch = true

	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: handleService recover from: %v, stack trace:\n%s\n",
				ic.superSpec.Name(), err, debug.Stack())
		}
	}()

	ic.reloadTraffic()

	return
}

func (ic *IngressController) handleServiceInstances(serviceInstances map[string]*spec.ServiceInstanceSpec) (continueWatch bool) {
	continueWatch = true

	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: handleServiceInstance recover from: %v, stack trace:\n%s\n",
				ic.superSpec.Name(), err, debug.Stack())
		}
	}()

	ic.reloadTraffic()

	return
}

func (ic *IngressController) reloadTraffic() {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	ic._reloadIngress()
	ic._reloadPipelines()
	ic._reloadHTTPServer()
}

func (ic *IngressController) _reloadIngress() {
	ingressBackends, ingressRules := make(map[string]struct{}), []*spec.IngressRule{}
	for _, ingress := range ic.service.ListIngressSpecs() {
		for _, rule := range ingress.Rules {
			for _, path := range rule.Paths {
				ingressBackends[path.Backend] = struct{}{}
				serviceSpec := &spec.Service{
					Name: path.Backend,
				}
				path.Backend = serviceSpec.IngressControllerPipelineName()
			}

			ingressRules = append(ingressRules, rule)
		}
	}

	ic.ingressBackends, ic.ingressRules = ingressBackends, ingressRules
}

func (ic *IngressController) _reloadPipelines() {
	for backend, entity := range ic.backendPipelines {
		if _, exists := ic.ingressBackends[backend]; !exists {
			err := ic.tc.DeletePipeline(ic.namespace, entity.Spec().Name())
			if err != nil {
				logger.Errorf("delete http pipeline %s failed: %v",
					entity.Spec().Name(), err)
			}
			delete(ic.backendPipelines, backend)
		}
	}

	// if in mTLS strict model, should init pipeline with certificates
	admSpec := ic.superSpec.ObjectSpec().(*spec.Admin)
	var cert, rootCert *spec.Certificate
	if admSpec.EnablemTLS() {
		cert = ic.service.GetIngressControllerInstanceCert(ic.instanceID)
		rootCert = ic.service.GetRootCert()
	}

	canaries := ic.service.ListServiceCanaries()
	for _, serviceSpec := range ic.service.ListServiceSpecs() {
		if _, exists := ic.ingressBackends[serviceSpec.BackendName()]; !exists {
			continue
		}

		instanceSpecs := ic.service.ListServiceInstanceSpecs(serviceSpec.Name)
		if len(instanceSpecs) == 0 {
			continue
		}
		upInstance := 0
		for _, instanceSpec := range instanceSpecs {
			if instanceSpec.Status == spec.ServiceStatusUp {
				upInstance++
			}
		}
		if upInstance == 0 {
			continue
		}

		superSpec, err := serviceSpec.IngressControllerPipelineSpec(instanceSpecs, canaries, cert, rootCert)
		if err != nil {
			logger.Errorf("get ingress pipeline for %s failed: %v",
				serviceSpec.Name, err)
			continue
		}

		entity, err := ic.tc.ApplyPipelineForSpec(ic.namespace, superSpec)
		if err != nil {
			logger.Errorf("apply http pipeline %s failed: %v", superSpec.Name(), err)
			continue
		}

		ic.backendPipelines[serviceSpec.BackendName()] = entity
	}
}

func (ic *IngressController) _reloadHTTPServer() {
	superSpec, err := spec.IngressControllerHTTPServerSpec(ic.spec.IngressPort, ic.ingressRules)
	if err != nil {
		logger.Errorf("get ingress http server spec failed: %v", err)
		return
	}

	entity, err := ic.tc.ApplyTrafficGateForSpec(ic.namespace, superSpec)
	if err != nil {
		logger.Errorf("apply http server failed: %v", err)
		return
	}

	ic.httpServer = entity
}

// Status returns the status of IngressController.
func (ic *IngressController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: struct{}{},
	}
}

// Close closes the ingress controller
func (ic *IngressController) Close() {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	ic.informer.Close()
	ic.tc.Clean(ic.namespace)
}

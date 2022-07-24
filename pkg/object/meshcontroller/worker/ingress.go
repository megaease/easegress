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

package worker

import (
	"fmt"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/supervisor"
)

// ErrIngressClosed is the error when operating in a closed Ingress server
var ErrIngressClosed = fmt.Errorf("ingress has been closed")

type (
	// IngressServer manages one ingress pipeline and one HTTPServer
	IngressServer struct {
		super       *supervisor.Supervisor
		superSpec   *supervisor.Spec
		serviceName string
		service     *service.Service

		mutex sync.RWMutex

		tc              *trafficcontroller.TrafficController
		applicationPort uint32
		namespace       string
		inf             informer.Informer
		instanceID      string

		pipelines  map[string]*supervisor.ObjectEntity
		httpServer *supervisor.ObjectEntity
	}
)

// NewIngressServer creates an initialized ingress server
func NewIngressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor,
	serviceName, instaceID string, service *service.Service,
) *IngressServer {
	entity, exists := super.GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}

	inf := informer.NewInformer(storage.New(superSpec.Name(), super.Cluster()), serviceName)

	return &IngressServer{
		super:     super,
		superSpec: superSpec,

		tc:        tc,
		namespace: superSpec.Name(),

		pipelines:   make(map[string]*supervisor.ObjectEntity),
		httpServer:  nil,
		serviceName: serviceName,
		instanceID:  instaceID,
		inf:         inf,
		mutex:       sync.RWMutex{},
		service:     service,
	}
}

// Ready checks ingress's pipeline and HTTPServer are created or not
func (ings *IngressServer) Ready() bool {
	ings.mutex.RLock()
	defer ings.mutex.RUnlock()

	return ings._ready()
}

func (ings *IngressServer) _ready() bool {
	serviceSpec := &spec.Service{
		Name: ings.serviceName,
	}

	_, pipelineReady := ings.pipelines[serviceSpec.SidecarIngressPipelineName()]

	return pipelineReady && (ings.httpServer != nil)
}

// InitIngress creates local default pipeline and httpServer for ingress
func (ings *IngressServer) InitIngress(service *spec.Service, port uint32) error {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.applicationPort = port

	if _, ok := ings.pipelines[service.SidecarIngressPipelineName()]; !ok {
		superSpec, err := service.SidecarIngressPipelineSpec(port)
		if err != nil {
			return err
		}
		entity, err := ings.tc.CreatePipelineForSpec(ings.namespace, superSpec)
		if err != nil {
			return fmt.Errorf("create http pipeline %s failed: %v", superSpec.Name(), err)
		}
		ings.pipelines[service.SidecarIngressPipelineName()] = entity
	}

	admSpec := ings.superSpec.ObjectSpec().(*spec.Admin)
	if ings.httpServer == nil {
		var cert, rootCert *spec.Certificate
		if admSpec.EnablemTLS() {
			cert = ings.service.GetServiceInstanceCert(ings.serviceName, ings.instanceID)
			rootCert = ings.service.GetRootCert()
			logger.Infof("ingress enable TLS, init httpserver with cert: %#v", cert)
		}

		superSpec, err := service.SidecarIngressHTTPServerSpec(admSpec.WorkerSpec.Ingress.KeepAlive,
			admSpec.WorkerSpec.Ingress.KeepAliveTimeout, cert, rootCert)
		if err != nil {
			return err
		}

		entity, err := ings.tc.CreateTrafficGateForSpec(ings.namespace, superSpec)
		if err != nil {
			return fmt.Errorf("create http server %s failed: %v", superSpec.Name(), err)
		}
		ings.httpServer = entity
	}

	if err := ings.inf.OnPartOfServiceSpec(service.Name, ings.reloadPipeline); err != nil {
		// Only return err when its type is not `AlreadyWatched`
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add ingress spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	if admSpec.EnablemTLS() {
		logger.Infof("ingress in mtls mode, start listen ID: %s's cert", ings.instanceID)
		if err := ings.inf.OnServerCert(ings.serviceName, ings.instanceID, ings.reloadHTTPServer); err != nil {
			if err != informer.ErrAlreadyWatched {
				logger.Errorf("add egress spec watching service: %s failed: %v", service.Name, err)
				return err
			}
		}
	}

	return nil
}

func (ings *IngressServer) reloadHTTPServer(event informer.Event, value *spec.Certificate) bool {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if event.EventType == informer.EventDelete {
		logger.Infof("receive delete event: %#v", event)
		return false
	}

	serviceSpec := ings.service.GetServiceSpec(ings.serviceName)
	if serviceSpec == nil {
		logger.Infof("ingress can't find its service: %s", ings.serviceName)
		return false
	}
	admSpec := ings.superSpec.ObjectSpec().(*spec.Admin)
	rootCert := ings.service.GetRootCert()
	superSpec, err := serviceSpec.SidecarIngressHTTPServerSpec(admSpec.WorkerSpec.Ingress.KeepAlive, admSpec.WorkerSpec.Ingress.KeepAliveTimeout, value, rootCert)
	if err != nil {
		logger.Errorf("BUG: update ingress pipeline spec: %s new super spec failed: %v",
			superSpec.YAMLConfig(), err)
		return true
	}

	entity, err := ings.tc.UpdateTrafficGateForSpec(ings.namespace, superSpec)
	if err != nil {
		logger.Errorf("update http server %s failed: %v", ings.serviceName, err)
		return true
	}

	// update local storage
	ings.httpServer = entity

	return true
}

func (ings *IngressServer) reloadPipeline(event informer.Event, serviceSpec *spec.Service) bool {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if event.EventType == informer.EventDelete {
		logger.Infof("receive delete event: %#v", event)
		return false
	}

	superSpec, err := serviceSpec.SidecarIngressPipelineSpec(ings.applicationPort)
	if err != nil {
		logger.Errorf("BUG: update ingress pipeline spec: %s new super spec failed: %v",
			superSpec.YAMLConfig(), err)
		return true
	}

	entity, err := ings.tc.UpdatePipelineForSpec(ings.namespace, superSpec)
	if err != nil {
		return true
	}

	ings.pipelines[ings.serviceName] = entity
	return true
}

// Close closes the Ingress HTTPServer and Pipeline
func (ings *IngressServer) Close() {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.inf.Close()

	if ings._ready() {
		ings.tc.DeleteTrafficGate(ings.namespace, ings.httpServer.Spec().Name())
		for _, entity := range ings.pipelines {
			ings.tc.DeletePipeline(ings.namespace, entity.Spec().Name())
		}
	}
}

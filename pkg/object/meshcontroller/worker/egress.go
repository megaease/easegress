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
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/supervisor"
	"gopkg.in/yaml.v2"
)

const egressRPCKey = "X-Mesh-Rpc-Service"

type (
	// EgressServer manages one/many ingress pipelines and one HTTPServer
	EgressServer struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec

		pipelines  map[string]*supervisor.ObjectEntity
		httpServer *supervisor.ObjectEntity

		tc         *trafficcontroller.TrafficController
		namespace  string
		inf        informer.Informer
		instanceID string

		chReloadEvent chan struct{}

		serviceName      string
		egressServerName string
		service          *service.Service
		mutex            sync.RWMutex
	}

	httpServerSpecBuilder struct {
		Kind            string `yaml:"kind"`
		Name            string `yaml:"name"`
		httpserver.Spec `yaml:",inline"`
		Cert            *spec.Certificate `yaml:"-"`
	}
)

// NewEgressServer creates an initialized egress server
func NewEgressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor,
	serviceName, instanceID string, service *service.Service,
) *EgressServer {
	entity, exists := super.GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}

	inf := informer.NewInformer(storage.New(superSpec.Name(), super.Cluster()), serviceName)

	return &EgressServer{
		super:     super,
		superSpec: superSpec,

		inf:         inf,
		tc:          tc,
		namespace:   superSpec.Name(),
		pipelines:   make(map[string]*supervisor.ObjectEntity),
		serviceName: serviceName,
		service:     service,
		instanceID:  instanceID,

		chReloadEvent: make(chan struct{}, 1),
	}
}

func newHTTPServerSpecBuilder(httpServerName string, spec *httpserver.Spec) *httpServerSpecBuilder {
	return &httpServerSpecBuilder{
		Kind: httpserver.Kind,
		Name: httpServerName,
		Spec: *spec,
	}
}

func (b *httpServerSpecBuilder) yamlConfig() string {
	buff, err := yaml.Marshal(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", b, err)
	}
	return string(buff)
}

// InitEgress initializes the Egress HTTPServer.
func (egs *EgressServer) InitEgress(service *spec.Service) error {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	if egs.httpServer != nil {
		return nil
	}

	egs.egressServerName = service.SidecarEgressServerName()
	admSpec := egs.superSpec.ObjectSpec().(*spec.Admin)
	superSpec, err := service.SidecarEgressHTTPServerSpec(admSpec.WorkerSpec.Egress.KeepAlive, admSpec.WorkerSpec.Egress.KeepAliveTimeout)
	if err != nil {
		return err
	}

	entity, err := egs.tc.CreateTrafficGateForSpec(egs.namespace, superSpec)
	if err != nil {
		return fmt.Errorf("create http server %s failed: %v", superSpec.Name(), err)
	}
	egs.httpServer = entity

	if err := egs.inf.OnAllServiceSpecs(egs.reloadBySpecs); err != nil {
		// only return err when its type is not `AlreadyWatched`
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add service spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	if err := egs.inf.OnAllServiceInstanceSpecs(egs.reloadByInstances); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add service instance spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	if admSpec.EnablemTLS() {
		logger.Infof("egress in mtls mode, start listen ID: %s's cert", egs.instanceID)
		if err := egs.inf.OnServerCert(egs.serviceName, egs.instanceID, egs.reloadByCert); err != nil {
			if err != informer.ErrAlreadyWatched {
				logger.Errorf("add server cert spec watching service: %s failed: %v", service.Name, err)
				return err
			}
		}
	}

	if err := egs.inf.OnAllHTTPRouteGroupSpecs(egs.reloadByHTTPRouteGroups); err != nil {
		// only return err when its type is not `AlreadyWatched`
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add HTTP route group spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	if err := egs.inf.OnAllTrafficTargetSpecs(egs.reloadByTrafficTargets); err != nil {
		// only return err when its type is not `AlreadyWatched`
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add traffic target spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	if err := egs.inf.OnAllServiceCanaries(egs.reloadByServiceCanaries); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add service canary watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	go egs.watch()

	return nil
}

// Ready checks Egress HTTPServer has been created or not.
// Not need to check pipelines, cause they will be dynamically added.
func (egs *EgressServer) Ready() bool {
	egs.mutex.RLock()
	defer egs.mutex.RUnlock()
	return egs._ready()
}

func (egs *EgressServer) _ready() bool {
	return egs.httpServer != nil
}

func (egs *EgressServer) reloadByCert(event informer.Event, value *spec.Certificate) bool {
	select {
	case egs.chReloadEvent <- struct{}{}:
	default:
	}
	return true
}

func (egs *EgressServer) reloadByInstances(value map[string]*spec.ServiceInstanceSpec) bool {
	select {
	case egs.chReloadEvent <- struct{}{}:
	default:
	}
	return true
}

func (egs *EgressServer) reloadBySpecs(value map[string]*spec.Service) bool {
	select {
	case egs.chReloadEvent <- struct{}{}:
	default:
	}
	return true
}

func (egs *EgressServer) reloadByHTTPRouteGroups(value map[string]*spec.HTTPRouteGroup) bool {
	select {
	case egs.chReloadEvent <- struct{}{}:
	default:
	}
	return true
}

func (egs *EgressServer) reloadByTrafficTargets(value map[string]*spec.TrafficTarget) bool {
	select {
	case egs.chReloadEvent <- struct{}{}:
	default:
	}
	return true
}

func (egs *EgressServer) reloadByServiceCanaries(value map[string]*spec.ServiceCanary) bool {
	select {
	case egs.chReloadEvent <- struct{}{}:
	default:
	}
	return true
}

func (egs *EgressServer) listTrafficTargets(lgSvcs map[string]*spec.Service) []*spec.TrafficTarget {
	var result []*spec.TrafficTarget

	tts := egs.service.ListTrafficTargets()
	for _, tt := range tts {
		// the destination service is a local or global service, which is already accessible
		if lgSvcs[tt.Destination.Name] != nil {
			continue
		}
		for _, s := range tt.Sources {
			if s.Name == egs.serviceName {
				result = append(result, tt)
				break
			}
		}
	}

	return result
}

func (egs *EgressServer) listHTTPRouteGroups(tts []*spec.TrafficTarget) map[string]*spec.HTTPRouteGroup {
	result := map[string]*spec.HTTPRouteGroup{}

	for _, tt := range tts {
		for _, r := range tt.Rules {
			if result[r.Name] != nil {
				continue
			}
			g := egs.service.GetHTTPRouteGroup(r.Name)
			if g != nil {
				result[g.Name] = g
			}
		}
	}

	return result
}

// listLocalAndGlobalServices returns services which can be accessed without a traffic control rule
func (egs *EgressServer) listLocalAndGlobalServices() map[string]*spec.Service {
	result := map[string]*spec.Service{}

	self := egs.service.GetServiceSpec(egs.serviceName)
	if self == nil {
		logger.Errorf("cannot find service: %s", egs.serviceName)
		return result
	}

	tenant := egs.service.GetTenantSpec(self.RegisterTenant)
	if tenant != nil {
		for _, name := range tenant.Services {
			if name == egs.serviceName {
				continue
			}

			spec := egs.service.GetServiceSpec(name)
			if spec != nil {
				result[name] = spec
			}
		}
	}

	if self.RegisterTenant == spec.GlobalTenant {
		return result
	}

	tenant = egs.service.GetTenantSpec(spec.GlobalTenant)
	if tenant == nil {
		return result
	}

	for _, name := range tenant.Services {
		if name == egs.serviceName {
			continue
		}
		if result[name] != nil {
			continue
		}
		spec := egs.service.GetServiceSpec(name)
		if spec != nil {
			result[name] = spec
		}
	}

	return result
}

func (egs *EgressServer) listServiceOfTrafficTarget(tts []*spec.TrafficTarget) map[string]*spec.Service {
	result := map[string]*spec.Service{}

	for _, tt := range tts {
		name := tt.Destination.Name
		if result[name] != nil {
			continue
		}
		spec := egs.service.GetServiceSpec(name)
		if spec == nil {
			logger.Errorf("cannot find service %s of traffic target %s", egs.serviceName, tt.Name)
			continue
		}
		result[name] = spec
	}

	return result
}

// regex rule: ^(\w+\.)*vet-services\.(\w+)\.svc\..+$
//  can match e.g. _tcp.vet-services.easemesh.svc.cluster.local
//   		   vet-services.easemesh.svc.cluster.local
//   		   _zip._tcp.vet-services.easemesh.svc.com
func (egs *EgressServer) buildHostRegex(serviceName string) string {
	return `^(\w+\.)*` + serviceName + `\.(\w+)\.svc\..+`
}

func (egs *EgressServer) buildMuxRule(pipelineName, serviceName string, matches []spec.HTTPMatch) []*httpserver.Rule {
	var rules []*httpserver.Rule
	headers := []*httpserver.Header{
		{
			Key: egressRPCKey,
			// Value should be the service name
			Values: []string{serviceName},
		},
	}

	for _, m := range matches {
		methods := m.Methods
		if len(methods) == 1 && methods[0] == "*" {
			methods = nil
		}

		rule := &httpserver.Rule{
			Paths: []*httpserver.Path{
				{
					Methods:    methods,
					PathRegexp: "^" + m.PathRegex,
					Headers:    headers,
					Backend:    pipelineName,
				},
			},
		}

		rules = append(rules, rule)
	}

	return rules
}

func (egs *EgressServer) listServiceInstances(serviceName string) []*spec.ServiceInstanceSpec {
	instances := egs.service.ListServiceInstanceSpecs(serviceName)
	for _, inst := range instances {
		if inst.Status == spec.ServiceStatusUp {
			return instances
		}
	}
	return nil
}

func (egs *EgressServer) reload() {
	lgSvcs := egs.listLocalAndGlobalServices()
	tts := egs.listTrafficTargets(lgSvcs)
	ttSvcs := egs.listServiceOfTrafficTarget(tts)
	groups := egs.listHTTPRouteGroups(tts)

	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	admSpec := egs.superSpec.ObjectSpec().(*spec.Admin)
	var cert, rootCert *spec.Certificate
	if admSpec.EnablemTLS() {
		cert = egs.service.GetServiceInstanceCert(egs.serviceName, egs.instanceID)
		rootCert = egs.service.GetRootCert()
		logger.Infof("egress enable TLS")
	}

	pipelines := make(map[string]*supervisor.ObjectEntity)
	serverName2PipelineName := make(map[string]string)

	canaries := egs.service.ListServiceCanaries()
	createPipeline := func(svc *spec.Service) {
		instances := egs.listServiceInstances(svc.Name)
		if len(instances) == 0 {
			logger.Warnf("service %s has no instance in UP status", svc.Name)
			return
		}

		pipelineSpec, err := svc.SidecarEgressPipelineSpec(instances, canaries, cert, rootCert)
		if err != nil {
			logger.Errorf("generate sidecar egress pipeline spec for service %s failed: %v", svc.Name, err)
			return
		}
		logger.Infof("service: %s visit: %s pipeline init ok", egs.serviceName, svc.Name)

		entity, err := egs.tc.CreatePipelineForSpec(egs.namespace, pipelineSpec)
		if err != nil {
			logger.Errorf("update http pipeline failed: %v", err)
			return
		}
		pipelines[svc.Name] = entity
		serverName2PipelineName[svc.Name] = pipelineSpec.Name()
	}

	for _, svc := range lgSvcs {
		createPipeline(svc)
	}
	for _, svc := range ttSvcs {
		createPipeline(svc)
	}

	httpServerSpec := egs.httpServer.Spec().ObjectSpec().(*httpserver.Spec)
	httpServerSpec.Rules = nil

	for serviceName := range lgSvcs {
		rule := &httpserver.Rule{
			Paths: []*httpserver.Path{
				{
					PathPrefix: "/",
					Headers: []*httpserver.Header{
						{
							Key: egressRPCKey,
							// Value should be the service name
							Values: []string{serviceName},
						},
					},
					// this name should be the pipeline full name
					Backend: serverName2PipelineName[serviceName],
				},
			},
		}

		// for matching only host name request
		//   1) try exactly matching
		//   2) try matching with regexp
		ruleHost := &httpserver.Rule{
			Host:       serviceName,
			HostRegexp: egs.buildHostRegex(serviceName),
			Paths: []*httpserver.Path{
				{
					PathPrefix: "/",
					// this name should be the pipeline full name
					Backend: serverName2PipelineName[serviceName],
				},
			},
		}

		httpServerSpec.Rules = append(httpServerSpec.Rules, rule, ruleHost)
	}

	for _, tt := range tts {
		pipelineName := serverName2PipelineName[tt.Destination.Name]
		if pipelineName == "" {
			continue
		}

		for _, r := range tt.Rules {
			var matches []spec.HTTPMatch
			if len(r.Matches) == 0 {
				matches = groups[r.Name].Matches
			} else {
				allMatches := groups[r.Name].Matches
				for _, name := range r.Matches {
					for i := range allMatches {
						if allMatches[i].Name == name {
							matches = append(matches, allMatches[i])
						}
					}
				}
			}

			rules := egs.buildMuxRule(pipelineName, tt.Destination.Name, matches)
			httpServerSpec.Rules = append(httpServerSpec.Rules, rules...)
		}
	}

	builder := newHTTPServerSpecBuilder(egs.egressServerName, httpServerSpec)
	superSpec, err := supervisor.NewSpec(builder.yamlConfig())
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", err)
		return
	}
	entity, err := egs.tc.UpdateTrafficGateForSpec(egs.namespace, superSpec)
	if err != nil {
		logger.Errorf("update http server %s failed: %v", egs.egressServerName, err)
		return
	}

	// update local storage
	egs.pipelines = pipelines
	egs.httpServer = entity
}

func (egs *EgressServer) watch() {
	for range egs.chReloadEvent {
		egs.reload()
	}
}

// Close closes the Egress HTTPServer and Pipelines
func (egs *EgressServer) Close() {
	close(egs.chReloadEvent)
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	egs.inf.Close()

	if egs._ready() {
		egs.tc.DeleteTrafficGate(egs.namespace, egs.httpServer.Spec().Name())
		for _, entity := range egs.pipelines {
			egs.tc.DeletePipeline(egs.namespace, entity.Spec().Name())
		}
	}
}

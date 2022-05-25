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

package informer

import (
	"fmt"
	"sync"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
)

const (
	// TODO: Support EventCreate.

	// EventUpdate is the update inform event.
	EventUpdate = "Update"
	// EventDelete is the delete inform event.
	EventDelete = "Delete"
)

type (
	// Event is the type of inform event.
	Event struct {
		EventType string
		RawKV     *mvccpb.KeyValue
	}

	specHandleFunc  func(event Event, value string) bool
	specsHandleFunc func(map[string]string) bool

	// The returning boolean flag of all callback functions means
	// if the stuff continues to be watched.

	// ServiceSpecFunc is the callback function type for service spec.
	ServiceSpecFunc func(event Event, serviceSpec *spec.Service) bool

	// ServiceSpecsFunc is the callback function type for service specs.
	ServiceSpecsFunc func(value map[string]*spec.Service) bool

	// ServicesInstanceSpecFunc is the callback function type for service instance spec.
	ServicesInstanceSpecFunc func(event Event, instanceSpec *spec.ServiceInstanceSpec) bool

	// ServiceInstanceSpecsFunc is the callback function type for service instance specs.
	ServiceInstanceSpecsFunc func(value map[string]*spec.ServiceInstanceSpec) bool

	// ServiceInstanceStatusFunc is the callback function type for service instance status.
	ServiceInstanceStatusFunc func(event Event, value *spec.ServiceInstanceStatus) bool

	// ServiceInstanceStatusesFunc is the callback function type for service instance statuses.
	ServiceInstanceStatusesFunc func(value map[string]*spec.ServiceInstanceStatus) bool

	// TenantSpecFunc is the callback function type for tenant spec.
	TenantSpecFunc func(event Event, value *spec.Tenant) bool

	// TenantSpecsFunc is the callback function type for tenant specs.
	TenantSpecsFunc func(value map[string]*spec.Tenant) bool

	// IngressSpecFunc is the callback function type for ingress spec.
	IngressSpecFunc func(event Event, ingressSpec *spec.Ingress) bool

	// IngressSpecsFunc is the callback function type for ingress specs.
	IngressSpecsFunc func(value map[string]*spec.Ingress) bool

	// ServiceCertsFunc is the callback function type for service certs.
	ServiceCertsFunc func(value map[string]*spec.Certificate) bool

	// CertFunc is the callback function type for service/ingressController's cert.
	CertFunc func(event Event, value *spec.Certificate) bool

	// HTTPRouteGroupSpecFunc is the callback function type for HTTP route group spec.
	HTTPRouteGroupSpecFunc func(event Event, value *spec.HTTPRouteGroup) bool

	// HTTPRouteGroupSpecsFunc is the callback function type for HTTP route group specs.
	HTTPRouteGroupSpecsFunc func(value map[string]*spec.HTTPRouteGroup) bool

	// TrafficTargetSpecFunc is the callback function type for traffic target spec.
	TrafficTargetSpecFunc func(event Event, value *spec.TrafficTarget) bool

	// TrafficTargetSpecsFunc is the callback function type for traffic target specs.
	TrafficTargetSpecsFunc func(value map[string]*spec.TrafficTarget) bool

	// ServiceCanarySpecFunc is the callback function type for service canary spec.
	ServiceCanarySpecFunc func(event Event, value *spec.ServiceCanary) bool

	// ServiceCanariesFunc is the callback function type for service canary specs.
	ServiceCanariesFunc func(value map[string]*spec.ServiceCanary) bool

	// Informer is the interface for informing two type of storage changed for every Mesh spec structure.
	//  1. Based on comparison between old and new part of entry.
	//  2. Based on comparison on entries with the same prefix.
	Informer interface {
		OnPartOfServiceSpec(serviceName string, fn ServiceSpecFunc) error
		OnAllServiceSpecs(fn ServiceSpecsFunc) error

		OnPartOfServiceInstanceSpec(serviceName, instanceID string, fn ServicesInstanceSpecFunc) error
		OnServiceInstanceSpecs(serviceName string, fn ServiceInstanceSpecsFunc) error
		OnAllServiceInstanceSpecs(fn ServiceInstanceSpecsFunc) error

		OnPartOfServiceInstanceStatus(serviceName, instanceID string, fn ServiceInstanceStatusFunc) error
		OnServiceInstanceStatuses(serviceName string, fn ServiceInstanceStatusesFunc) error
		OnAllServiceInstanceStatuses(fn ServiceInstanceStatusesFunc) error

		OnPartOfTenantSpec(tenantName string, fn TenantSpecFunc) error
		OnAllTenantSpecs(fn TenantSpecsFunc) error

		OnPartOfIngressSpec(serviceName string, fn IngressSpecFunc) error
		OnAllIngressSpecs(fn IngressSpecsFunc) error

		OnPartOfHTTPRouteGroupSpec(groupName string, fn HTTPRouteGroupSpecFunc) error
		OnAllHTTPRouteGroupSpecs(fn HTTPRouteGroupSpecsFunc) error

		OnPartOfTrafficTargetSpec(ttName string, fn TrafficTargetSpecFunc) error
		OnAllTrafficTargetSpecs(fn TrafficTargetSpecsFunc) error

		OnPartOfServiceCanary(serviceCanaryName string, fn ServiceCanarySpecFunc) error
		OnAllServiceCanaries(fn ServiceCanariesFunc) error

		StopWatchServiceSpec(serviceName string)
		StopWatchServiceInstanceSpec(serviceName string)

		OnAllServerCert(fn ServiceCertsFunc) error
		OnServerCert(serviceName, instanceID string, fn CertFunc) error
		OnIngressControllerCert(instaceID string, fn CertFunc) error

		Close()
	}

	// meshInformer is the informer for mesh usage
	meshInformer struct {
		mutex   sync.RWMutex
		store   storage.Storage
		syncers map[string]cluster.Syncer

		service         string
		globalServices  map[string]bool   // name of service in global tenant
		service2Tenants map[string]string // service name to its registered tenant

		closed bool
		done   chan struct{}
	}
)

var (
	// ErrAlreadyWatched is the error when watch the same entry again.
	ErrAlreadyWatched = fmt.Errorf("already watched")

	// ErrClosed is the error when watching a closed informer.
	ErrClosed = fmt.Errorf("informer already been closed")

	// ErrNotFound is the error when watching an entry which is not found.
	ErrNotFound = fmt.Errorf("not found")
)

// NewInformer creates an informer
// If service is specified, will only inform resource changes within the same tenant
// of the service and the global tenant, note this only apply to service, service instance
// and service status.
// if service is empty, will inform all resource changes.
func NewInformer(store storage.Storage, service string) Informer {
	inf := &meshInformer{
		store:           store,
		syncers:         make(map[string]cluster.Syncer),
		done:            make(chan struct{}),
		service:         service,
		globalServices:  make(map[string]bool),
		service2Tenants: make(map[string]string),
	}

	// empty service name means we won't filter data by tenant
	if len(service) == 0 {
		return inf
	}

	storeKey := layout.ServiceSpecPrefix()
	services, err := inf.store.GetPrefix(storeKey)
	if err != nil {
		logger.Errorf("failed to load service specs: %v", err)
		return inf
	}
	inf.buildServiceToTenantMap(services)

	syncerKey := "informer-service"
	inf.onSpecs(storeKey, syncerKey, inf.buildServiceToTenantMap)

	storeKey = layout.TenantSpecKey(spec.GlobalTenant)
	tenants, err := inf.store.GetPrefix(storeKey)
	if err != nil {
		logger.Errorf("failed to load tenant specs: %v", err)
		return inf
	}
	inf.updateGlobalServices(tenants)

	syncerKey = "informer-global-tenant"
	inf.onSpecs(storeKey, syncerKey, inf.updateGlobalServices)

	return inf
}

func (inf *meshInformer) updateGlobalServices(kvs map[string]string) bool {
	var tenant *spec.Tenant
	for _, v := range kvs {
		t := &spec.Tenant{}
		if err := yaml.Unmarshal([]byte(v), t); err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		if t.Name == spec.GlobalTenant {
			tenant = t
			break
		}
	}
	if tenant == nil {
		return true
	}

	services := make(map[string]bool, len(tenant.Services))
	for _, s := range tenant.Services {
		services[s] = true
	}

	inf.mutex.Lock()
	inf.globalServices = services
	inf.mutex.Unlock()
	return true
}

func (inf *meshInformer) buildServiceToTenantMap(kvs map[string]string) bool {
	s2t := make(map[string]string, len(kvs))
	for _, v := range kvs {
		service := &spec.Service{}
		if err := yaml.Unmarshal([]byte(v), service); err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		s2t[service.Name] = service.RegisterTenant
	}

	if _, ok := s2t[inf.service]; !ok {
		logger.Errorf("BUG: need to get tenant of service %s, but the service does not exist", inf.service)
	}

	inf.mutex.Lock()
	inf.service2Tenants = s2t
	inf.mutex.Unlock()
	return true
}

func (inf *meshInformer) stopSyncOneKey(key string) {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if syncer, exists := inf.syncers[key]; exists {
		syncer.Close()
		delete(inf.syncers, key)
	}
}

func serviceSpecSyncerKey(serviceName string) string {
	return fmt.Sprintf("service-spec-%s", serviceName)
}

// OnPartOfServiceSpec watches one service's spec
func (inf *meshInformer) OnPartOfServiceSpec(serviceName string, fn ServiceSpecFunc) error {
	storeKey := layout.ServiceSpecKey(serviceName)
	syncerKey := serviceSpecSyncerKey(serviceName)

	specFunc := func(event Event, value string) bool {
		serviceSpec := &spec.Service{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), serviceSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, serviceSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

func (inf *meshInformer) StopWatchServiceSpec(serviceName string) {
	syncerKey := serviceSpecSyncerKey(serviceName)
	inf.stopSyncOneKey(syncerKey)
}

// OnPartOfServiceInstanceSpec watches one service's instance spec
func (inf *meshInformer) OnPartOfServiceInstanceSpec(serviceName, instanceID string, fn ServicesInstanceSpecFunc) error {
	storeKey := layout.ServiceInstanceSpecKey(serviceName, instanceID)
	syncerKey := fmt.Sprintf("service-instance-spec-%s-%s", serviceName, instanceID)

	specFunc := func(event Event, value string) bool {
		instanceSpec := &spec.ServiceInstanceSpec{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), instanceSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, instanceSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

// OnPartOfServiceInstanceStatus watches one service instance status spec
func (inf *meshInformer) OnPartOfServiceInstanceStatus(serviceName, instanceID string, fn ServiceInstanceStatusFunc) error {
	storeKey := layout.ServiceInstanceStatusKey(serviceName, instanceID)
	syncerKey := fmt.Sprintf("service-instance-status-%s-%s", serviceName, instanceID)

	specFunc := func(event Event, value string) bool {
		instanceStatus := &spec.ServiceInstanceStatus{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), instanceStatus); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, instanceStatus)
	}

	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

// OnPartOfTenantSpec watches one tenant spec
func (inf *meshInformer) OnPartOfTenantSpec(tenant string, fn TenantSpecFunc) error {
	storeKey := layout.TenantSpecKey(tenant)
	syncerKey := fmt.Sprintf("tenant-%s", tenant)

	specFunc := func(event Event, value string) bool {
		tenantSpec := &spec.Tenant{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), tenantSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, tenantSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

// OnPartOfIngressSpec watches one ingress spec
func (inf *meshInformer) OnPartOfIngressSpec(ingress string, fn IngressSpecFunc) error {
	storeKey := layout.IngressSpecKey(ingress)
	syncerKey := fmt.Sprintf("ingress-%s", ingress)

	specFunc := func(event Event, value string) bool {
		ingressSpec := &spec.Ingress{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), ingressSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, ingressSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

// OnPartOfHTTPRouteGroupSpec watches one HTTP route group spec
func (inf *meshInformer) OnPartOfHTTPRouteGroupSpec(group string, fn HTTPRouteGroupSpecFunc) error {
	storeKey := layout.HTTPRouteGroupKey(group)
	syncerKey := fmt.Sprintf("http-route-group-%s", group)

	specFunc := func(event Event, value string) bool {
		groupSpec := &spec.HTTPRouteGroup{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), groupSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, groupSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

// OnPartOfTrafficTargetSpec watches one traffic target spec
func (inf *meshInformer) OnPartOfTrafficTargetSpec(tt string, fn TrafficTargetSpecFunc) error {
	storeKey := layout.TrafficTargetKey(tt)
	syncerKey := fmt.Sprintf("traffic-target-%s", tt)

	specFunc := func(event Event, value string) bool {
		ttSpec := &spec.TrafficTarget{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), ttSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, ttSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

// OnPartOfServiceCanary watches one service canary.
func (inf *meshInformer) OnPartOfServiceCanary(servicecanaryName string, fn ServiceCanarySpecFunc) error {
	storeKey := layout.ServiceCanaryKey(servicecanaryName)
	syncerKey := fmt.Sprintf("service-canary-%s", servicecanaryName)

	specFunc := func(event Event, value string) bool {
		serviceCanary := &spec.ServiceCanary{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), serviceCanary); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, serviceCanary)
	}

	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

// OnAllServiceSpecs watches all service specs
func (inf *meshInformer) OnAllServiceSpecs(fn ServiceSpecsFunc) error {
	storeKey := layout.ServiceSpecPrefix()
	syncerKey := "prefix-service"

	specsFunc := func(kvs map[string]string) bool {
		inf.mutex.RLock()
		gs := inf.globalServices
		s2t := inf.service2Tenants
		inf.mutex.RUnlock()

		var tenant string
		if len(inf.service) > 0 && !gs[inf.service] {
			tenant = s2t[inf.service]
		}

		services := make(map[string]*spec.Service)
		for k, v := range kvs {
			service := &spec.Service{}
			if err := yaml.Unmarshal([]byte(v), service); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			if len(tenant) == 0 || gs[service.Name] || service.RegisterTenant == tenant {
				services[k] = service
			}
		}

		return fn(services)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

func serviceInstanceSpecSyncerKey(serviceName string) string {
	return fmt.Sprintf("prefix-service-instance-spec-%s", serviceName)
}

func (inf *meshInformer) onServiceInstanceSpecs(storeKey, syncerKey string, fn ServiceInstanceSpecsFunc) error {
	specsFunc := func(kvs map[string]string) bool {
		inf.mutex.RLock()
		gs := inf.globalServices
		s2t := inf.service2Tenants
		inf.mutex.RUnlock()

		var tenant string
		if len(inf.service) > 0 && !gs[inf.service] {
			tenant = s2t[inf.service]
		}

		instanceSpecs := make(map[string]*spec.ServiceInstanceSpec)
		for k, v := range kvs {
			instanceSpec := &spec.ServiceInstanceSpec{}
			if err := yaml.Unmarshal([]byte(v), instanceSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			if len(tenant) == 0 || gs[instanceSpec.ServiceName] || s2t[instanceSpec.ServiceName] == tenant {
				instanceSpecs[k] = instanceSpec
			}
		}

		return fn(instanceSpecs)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

// OnServiceInstanceSpecs watches all instance specs of a service.
func (inf *meshInformer) OnServiceInstanceSpecs(serviceName string, fn ServiceInstanceSpecsFunc) error {
	storeKey := layout.ServiceInstanceSpecPrefix(serviceName)
	syncerKey := serviceInstanceSpecSyncerKey(serviceName)
	return inf.onServiceInstanceSpecs(storeKey, syncerKey, fn)
}

// OnAllServiceInstanceSpecs watches instance specs of all services.
func (inf *meshInformer) OnAllServiceInstanceSpecs(fn ServiceInstanceSpecsFunc) error {
	storeKey := layout.AllServiceInstanceSpecPrefix()
	syncerKey := "prefix-service-instance"
	return inf.onServiceInstanceSpecs(storeKey, syncerKey, fn)
}

func (inf *meshInformer) StopWatchServiceInstanceSpec(serviceName string) {
	syncerKey := serviceInstanceSpecSyncerKey(serviceName)
	inf.stopSyncOneKey(syncerKey)
}

func (inf *meshInformer) onServiceInstanceStatuses(storeKey, syncerKey string, fn ServiceInstanceStatusesFunc) error {
	specsFunc := func(kvs map[string]string) bool {
		inf.mutex.RLock()
		gs := inf.globalServices
		s2t := inf.service2Tenants
		inf.mutex.RUnlock()

		var tenant string
		if len(inf.service) > 0 && !gs[inf.service] {
			tenant = s2t[inf.service]
		}

		instanceStatuses := make(map[string]*spec.ServiceInstanceStatus)
		for k, v := range kvs {
			instanceStatus := &spec.ServiceInstanceStatus{}
			if err := yaml.Unmarshal([]byte(v), instanceStatus); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			if len(tenant) == 0 || gs[instanceStatus.ServiceName] || s2t[instanceStatus.ServiceName] == tenant {
				instanceStatuses[k] = instanceStatus
			}
		}

		return fn(instanceStatuses)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

// OnServiceInstanceStatuses watches instance statuses of a service
func (inf *meshInformer) OnServiceInstanceStatuses(serviceName string, fn ServiceInstanceStatusesFunc) error {
	storeKey := layout.ServiceInstanceStatusPrefix(serviceName)
	syncerKey := fmt.Sprintf("prefix-service-instance-status-%s", serviceName)
	return inf.onServiceInstanceStatuses(storeKey, syncerKey, fn)
}

// OnAllServiceInstanceStatuses watches instance statuses of all services
func (inf *meshInformer) OnAllServiceInstanceStatuses(fn ServiceInstanceStatusesFunc) error {
	storeKey := layout.AllServiceInstanceStatusPrefix()
	syncerKey := "prefix-service-instance-status"
	return inf.onServiceInstanceStatuses(storeKey, syncerKey, fn)
}

// OnAllTenantSpecs watches all tenant specs
func (inf *meshInformer) OnAllTenantSpecs(fn TenantSpecsFunc) error {
	storeKey := layout.TenantPrefix()
	syncerKey := "prefix-tenant"

	specsFunc := func(kvs map[string]string) bool {
		tenants := make(map[string]*spec.Tenant)
		for k, v := range kvs {
			tenantSpec := &spec.Tenant{}
			if err := yaml.Unmarshal([]byte(v), tenantSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			tenants[k] = tenantSpec
		}

		return fn(tenants)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

// OnAllIngressSpecs watches all ingress specs
func (inf *meshInformer) OnAllIngressSpecs(fn IngressSpecsFunc) error {
	storeKey := layout.IngressPrefix()
	syncerKey := "prefix-ingress"

	specsFunc := func(kvs map[string]string) bool {
		ingresss := make(map[string]*spec.Ingress)
		for k, v := range kvs {
			ingressSpec := &spec.Ingress{}
			if err := yaml.Unmarshal([]byte(v), ingressSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			ingresss[k] = ingressSpec
		}

		return fn(ingresss)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

func (inf *meshInformer) onCert(storeKey, syncerKey string, fn CertFunc) error {
	specFunc := func(event Event, value string) bool {
		cert := &spec.Certificate{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), cert); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
				return true
			}
		}
		return fn(event, cert)
	}
	return inf.onSpecPart(storeKey, syncerKey, specFunc)
}

func (inf *meshInformer) OnIngressControllerCert(instanceID string, fn CertFunc) error {
	storeKey := layout.IngressControllerInstanceCertKey(instanceID)
	syncerKey := fmt.Sprintf("ingresscontroller-%s-cert", instanceID)
	return inf.onCert(storeKey, syncerKey, fn)
}

func (inf *meshInformer) OnServerCert(serviceName, instanceID string, fn CertFunc) error {
	storeKey := layout.ServiceInstanceCertKey(serviceName, instanceID)
	syncerKey := fmt.Sprintf("service-%s-%s-cert", serviceName, instanceID)

	return inf.onCert(storeKey, syncerKey, fn)
}

// OnAllServerCert watches all service cert specs.
func (inf *meshInformer) OnAllServerCert(fn ServiceCertsFunc) error {
	storeKey := layout.AllServiceCertPrefix()
	syncerKey := "prefix-certs"

	specsFunc := func(kvs map[string]string) bool {
		cert := make(map[string]*spec.Certificate)
		for k, v := range kvs {
			certSpec := &spec.Certificate{}
			if err := yaml.Unmarshal([]byte(v), certSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			cert[k] = certSpec
		}

		return fn(cert)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

// OnAllHTTPRouteGroupSpecs watches all http route specs.
func (inf *meshInformer) OnAllHTTPRouteGroupSpecs(fn HTTPRouteGroupSpecsFunc) error {
	storeKey := layout.HTTPRouteGroupPrefix()
	syncerKey := "http-route-group-target"

	specsFunc := func(kvs map[string]string) bool {
		groups := make(map[string]*spec.HTTPRouteGroup)
		for k, v := range kvs {
			groupSpec := &spec.HTTPRouteGroup{}
			if err := yaml.Unmarshal([]byte(v), groupSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			groups[k] = groupSpec
		}

		return fn(groups)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

// OnAllTrafficTargetSpecs watches all traffic target specs.
func (inf *meshInformer) OnAllTrafficTargetSpecs(fn TrafficTargetSpecsFunc) error {
	storeKey := layout.TrafficTargetPrefix()
	syncerKey := "prefix-traffic-target"

	specsFunc := func(kvs map[string]string) bool {
		tts := make(map[string]*spec.TrafficTarget)
		for k, v := range kvs {
			ttSpec := &spec.TrafficTarget{}
			if err := yaml.Unmarshal([]byte(v), ttSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			tts[k] = ttSpec
		}

		return fn(tts)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

// OnAllServiceCanaries watches all service canary specs.
func (inf *meshInformer) OnAllServiceCanaries(fn ServiceCanariesFunc) error {
	storeKey := layout.ServiceCanaryPrefix()
	syncerKey := "prefix-service-canary"

	specsFunc := func(kvs map[string]string) bool {
		serviceCanaries := make(map[string]*spec.ServiceCanary)
		for k, v := range kvs {
			serviceCanary := &spec.ServiceCanary{}
			if err := yaml.Unmarshal([]byte(v), serviceCanary); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			serviceCanaries[k] = serviceCanary
		}

		return fn(serviceCanaries)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

// also need to rename this function and all its caller functions
// as they are not accurate anymore
func (inf *meshInformer) onSpecPart(storeKey, syncerKey string, fn specHandleFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if inf.closed {
		return ErrClosed
	}

	if _, ok := inf.syncers[syncerKey]; ok {
		logger.Infof("sync key: %s already", syncerKey)
		return ErrAlreadyWatched
	}

	syncer, err := inf.store.Syncer()
	if err != nil {
		return err
	}

	ch, err := syncer.SyncRaw(storeKey)
	if err != nil {
		return err
	}

	inf.syncers[syncerKey] = syncer

	go inf.sync(ch, syncerKey, fn)

	return nil
}

func (inf *meshInformer) onSpecs(storePrefix, syncerKey string, fn specsHandleFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if inf.closed {
		return ErrClosed
	}

	if _, exists := inf.syncers[syncerKey]; exists {
		logger.Infof("sync prefix:%s already", syncerKey)
		return ErrAlreadyWatched
	}

	syncer, err := inf.store.Syncer()
	if err != nil {
		return err
	}

	ch, err := syncer.SyncPrefix(storePrefix)
	if err != nil {
		return err
	}

	inf.syncers[syncerKey] = syncer

	go inf.syncPrefix(ch, syncerKey, fn)

	return nil
}

func (inf *meshInformer) Close() {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	for _, syncer := range inf.syncers {
		syncer.Close()
	}

	inf.closed = true
}

func (inf *meshInformer) sync(ch <-chan *mvccpb.KeyValue, syncerKey string, fn specHandleFunc) {
	for kv := range ch {
		var (
			event Event
			value string
		)

		if kv == nil {
			event.EventType = EventDelete
		} else {
			event.EventType = EventUpdate
			event.RawKV = kv
			value = string(kv.Value)
		}

		if !fn(event, value) {
			inf.stopSyncOneKey(syncerKey)
		}
	}
}

func (inf *meshInformer) syncPrefix(ch <-chan map[string]string, syncerKey string, fn specsHandleFunc) {
	for kvs := range ch {
		if !fn(kvs) {
			inf.stopSyncOneKey(syncerKey)
		}
	}
}

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

package consulserviceregistry

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of ConsulServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of ConsulServiceRegistry.
	Kind = "ConsulServiceRegistry"

	// MetaKeyRegistryName is the key of service metadata.
	// NOTE: Namespace is only available for Consul Enterprise,
	// instead we use this field to work around.
	MetaKeyRegistryName = "RegistryName"
)

func init() {
	supervisor.Register(&ConsulServiceRegistry{})
}

type (
	// ConsulServiceRegistry is Object ConsulServiceRegistry.
	ConsulServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		instances       map[string]*serviceregistry.ServiceInstanceSpec
		notify          chan *serviceregistry.RegistryEvent

		clientMutex sync.RWMutex
		client      consulClient

		statusMutex  sync.Mutex
		instancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the ConsulServiceRegistry.
	Spec struct {
		Address      string   `yaml:"address" jsonschema:"required"`
		Scheme       string   `yaml:"scheme" jsonschema:"required,enum=http,enum=https"`
		Datacenter   string   `yaml:"datacenter" jsonschema:"omitempty"`
		Token        string   `yaml:"token" jsonschema:"omitempty"`
		Namespace    string   `yaml:"namespace" jsonschema:"omitempty"`
		SyncInterval string   `yaml:"syncInterval" jsonschema:"required,format=duration"`
		ServiceTags  []string `yaml:"serviceTags" jsonschema:"omitempty"`
	}

	// Status is the status of ConsulServiceRegistry.
	Status struct {
		Health              string         `yaml:"health"`
		ServiceInstancesNum map[string]int `yaml:"instancesNum"`
	}
)

// Category returns the category of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		Address:      "127.0.0.1:8500",
		Scheme:       "http",
		SyncInterval: "10s",
	}
}

// Init initializes ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Init(superSpec *supervisor.Spec) {
	c.superSpec, c.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	c.reload()
}

// Inherit inherits previous generation of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	c.Init(superSpec)
}

func (c *ConsulServiceRegistry) reload() {
	c.serviceRegistry = c.superSpec.Super().MustGetSystemController(serviceregistry.Kind).
		Instance().(*serviceregistry.ServiceRegistry)
	c.notify = make(chan *serviceregistry.RegistryEvent, 10)
	c.firstDone = false

	c.instancesNum = map[string]int{}
	c.done = make(chan struct{})

	_, err := c.getClient()
	if err != nil {
		logger.Errorf("%s get consul client failed: %v", c.superSpec.Name(), err)
	}

	c.serviceRegistry.RegisterRegistry(c)

	go c.run()
}

func (c *ConsulServiceRegistry) getClient() (consulClient, error) {
	c.clientMutex.RLock()
	if c.client != nil {
		client := c.client
		c.clientMutex.RUnlock()
		return client, nil
	}
	c.clientMutex.RUnlock()

	return c.buildClient()
}

func (c *ConsulServiceRegistry) buildClient() (consulClient, error) {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	// DCL
	if c.client != nil {
		return c.client, nil
	}

	config := api.DefaultConfig()
	config.Address = c.spec.Address
	if config.Scheme != "" {
		config.Scheme = c.spec.Scheme
	}
	if config.Datacenter != "" {
		config.Datacenter = c.spec.Datacenter
	}
	if config.Token != "" {
		config.Token = c.spec.Token
	}

	if config.Namespace != "" {
		config.Namespace = c.spec.Namespace
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	c.client = newConsulAPIClient(client)

	return c.client, nil
}

func (c *ConsulServiceRegistry) closeClient() {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	if c.client == nil {
		return
	}

	c.client = nil
}

func (c *ConsulServiceRegistry) run() {
	defer c.closeClient()

	syncInterval, err := time.ParseDuration(c.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			c.spec.SyncInterval, err)
		return
	}

	c.update()

	for {
		select {
		case <-c.done:
			return
		case <-time.After(syncInterval):
			c.update()
		}
	}
}

func (c *ConsulServiceRegistry) update() {
	instances, err := c.ListAllServiceInstances()
	if err != nil {
		logger.Errorf("list all service instances failed: %v", err)
		return
	}

	instancesNum := make(map[string]int)
	for _, instance := range instances {
		instancesNum[instance.ServiceName]++
	}

	var event *serviceregistry.RegistryEvent
	if !c.firstDone {
		c.firstDone = true
		event = &serviceregistry.RegistryEvent{
			SourceRegistryName: c.Name(),
			UseReplace:         true,
			Replace:            instances,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(c.Name(), c.instances, instances)
	}

	if event.Empty() {
		return
	}

	c.notify <- event
	c.instances = instances

	c.statusMutex.Lock()
	c.instancesNum = instancesNum
	c.statusMutex.Unlock()
}

// Status returns status of ConsulServiceRegister.
func (c *ConsulServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := c.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	c.statusMutex.Lock()
	serversNum := c.instancesNum
	c.statusMutex.Unlock()

	s.ServiceInstancesNum = serversNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Close() {
	c.serviceRegistry.DeregisterRegistry(c.Name())

	if c.superSpec.Super().Cluster().IsLeader() {
		c.cleanExternalRegistry()
	}

	close(c.done)
}

func (c *ConsulServiceRegistry) cleanExternalRegistry() {
	instancesToDelete := map[string]*serviceregistry.ServiceInstanceSpec{}

	instances, err := c.ListAllServiceInstances()
	if err != nil {
		logger.Errorf("list all service instances failed: %v", err)
		return
	}

	for _, instance := range instances {
		if instance.RegistryName != c.Name() {
			instancesToDelete[instance.Key()] = instance
			logger.Infof("clean %s", instance.Key())
		}
	}
	err = c.DeleteServiceInstances(instancesToDelete)
	if err != nil {
		logger.Errorf("delete service instances %+v failed: %v",
			instancesToDelete, err)
	}
}

// Name returns name.
func (c *ConsulServiceRegistry) Name() string {
	return c.superSpec.Name()
}

// Notify returns notify channel.
func (c *ConsulServiceRegistry) Notify() <-chan *serviceregistry.RegistryEvent {
	return c.notify
}

// ApplyServiceInstances applies service instances to the registry.
func (c *ConsulServiceRegistry) ApplyServiceInstances(instances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := c.getClient()
	if err != nil {
		return fmt.Errorf("%s get consul client failed: %v",
			c.superSpec.Name(), err)
	}

	for _, instance := range instances {
		err := instance.Validate()
		if err != nil {
			return fmt.Errorf("%+v is invalid: %v", instance, err)
		}
	}

	for _, instance := range instances {
		registration := c.serviceInstanceToRegistration(instance)
		err = client.ServiceRegister(registration)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteServiceInstances applies service instances to the registry.
func (c *ConsulServiceRegistry) DeleteServiceInstances(instances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := c.getClient()
	if err != nil {
		return fmt.Errorf("%s get consul client failed: %v",
			c.superSpec.Name(), err)
	}

	for _, instance := range instances {
		err := client.ServiceDeregister(instance.InstanceID)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetServiceInstance get service instance from the registry.
func (c *ConsulServiceRegistry) GetServiceInstance(serviceName, instanceID string) (*serviceregistry.ServiceInstanceSpec, error) {
	instances, err := c.ListServiceInstances(serviceName)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		if instance.InstanceID == instanceID {
			return instance, nil
		}
	}

	return nil, fmt.Errorf("%s/%s not found", serviceName, instanceID)
}

// ListServiceInstances list service instances of one service from the registry.
func (c *ConsulServiceRegistry) ListServiceInstances(serviceName string) (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get consul client failed: %v",
			c.superSpec.Name(), err)
	}

	catalogServices, err := client.ListServiceInstances(serviceName)
	if err != nil {
		return nil, err
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, catalog := range catalogServices {
		serviceInstance := c.catalogServiceToServiceInstance(catalog)
		err := serviceInstance.Validate()
		if err != nil {
			return nil, fmt.Errorf("%+v is invalid: %v", serviceInstance, err)
		}

		instances[serviceInstance.Key()] = serviceInstance
	}

	return instances, nil
}

// ListAllServiceInstances list all service instances from the registry.
func (c *ConsulServiceRegistry) ListAllServiceInstances() (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get consul client failed: %v",
			c.superSpec.Name(), err)
	}

	catalogServices, err := client.ListAllServiceInstances()
	if err != nil {
		return nil, fmt.Errorf("%s pull catalog services failed: %v",
			c.superSpec.Name(), err)
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, catalogService := range catalogServices {
		serviceInstance := c.catalogServiceToServiceInstance(catalogService)
		if err := serviceInstance.Validate(); err != nil {
			return nil, fmt.Errorf("%+v is invalid: %v", serviceInstance, err)
		}

		instances[serviceInstance.Key()] = serviceInstance
	}

	return instances, nil
}

func (c *ConsulServiceRegistry) serviceInstanceToRegistration(serviceInstance *serviceregistry.ServiceInstanceSpec) *api.AgentServiceRegistration {
	return &api.AgentServiceRegistration{
		Kind:    api.ServiceKindTypical,
		ID:      serviceInstance.InstanceID,
		Name:    serviceInstance.ServiceName,
		Tags:    serviceInstance.Tags,
		Port:    int(serviceInstance.Port),
		Address: serviceInstance.Address,
		Meta: map[string]string{
			MetaKeyRegistryName: serviceInstance.RegistryName,
		},
	}
}

func (c *ConsulServiceRegistry) catalogServiceToServiceInstance(catalogService *api.CatalogService) *serviceregistry.ServiceInstanceSpec {
	registryName := c.Name()
	if catalogService.ServiceMeta != nil &&
		catalogService.ServiceMeta[MetaKeyRegistryName] != "" {
		registryName = catalogService.ServiceMeta[MetaKeyRegistryName]
	}

	serviceAddress := catalogService.ServiceAddress
	if serviceAddress == "" {
		serviceAddress = catalogService.Address
	}

	return &serviceregistry.ServiceInstanceSpec{
		RegistryName: registryName,
		ServiceName:  catalogService.ServiceName,
		InstanceID:   catalogService.ServiceID,
		Port:         uint16(catalogService.ServicePort),
		Tags:         catalogService.ServiceTags,
		Address:      serviceAddress,
	}
}

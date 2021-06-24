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

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/filter/proxy"
	"github.com/megaease/easegress/pkg/filter/requestadaptor"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/function/spec"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/supervisor"
)

const ingressFunctionKey = "X-FaaS-Func-Name"

type (
	// ingressServer manages one/many ingress pipelines and one HTTPServer
	ingressServer struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec

		faasNetworkLayerURL string
		faasHostSuffix      string
		faasNamespace       string

		namespace string
		mutex     sync.RWMutex

		tc             *trafficcontroller.TrafficController
		pipelines      map[string]struct{}
		httpServer     *supervisor.ObjectEntity
		httpServerSpec *supervisor.Spec
	}

	pipelineSpecBuilder struct {
		Kind              string `yaml:"kind"`
		Name              string `yaml:"name"`
		httppipeline.Spec `yaml:",inline"`
	}
)

// newIngressServer creates a initialized ingress server
func newIngressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor,
	controllerName string) *ingressServer {
	entity, exists := super.GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}
	return &ingressServer{
		pipelines:  make(map[string]struct{}),
		httpServer: nil,
		super:      super,
		superSpec:  superSpec,
		mutex:      sync.RWMutex{},
		namespace:  fmt.Sprintf("%s/%s", superSpec.Name(), "ingress"),
		tc:         tc,
	}
}

func newPipelineSpecBuilder(funcName string) *pipelineSpecBuilder {
	return &pipelineSpecBuilder{
		Kind: httppipeline.Kind,
		Name: funcName,
		Spec: httppipeline.Spec{},
	}
}

func (b *pipelineSpecBuilder) yamlConfig() string {
	buff, err := yaml.Marshal(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", b, err)
	}
	return string(buff)
}

func (b *pipelineSpecBuilder) appendReqAdaptor(funcSpec *spec.Spec, faasNamespace, faasHostSuffix string) *pipelineSpecBuilder {
	adaptorName := "requestAdaptor"
	b.Flow = append(b.Flow, httppipeline.Flow{Filter: adaptorName})

	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":   requestadaptor.Kind,
		"name":   adaptorName,
		"method": funcSpec.RequestAdaptor.Method,
		"path":   funcSpec.RequestAdaptor.Path,
		"header": funcSpec.RequestAdaptor.Header,

		// let faas Provider's gateway recognized this function by Host field
		"host": funcSpec.Name + "." + faasNamespace + "." + faasHostSuffix,
	})

	return b
}

func (b *pipelineSpecBuilder) appendProxy(faasNetworkLayerURL string) *pipelineSpecBuilder {
	mainServers := []*proxy.Server{
		{
			URL: faasNetworkLayerURL,
		},
	}

	backendName := "faasBackend"

	lb := &proxy.LoadBalance{
		Policy: proxy.PolicyRoundRobin,
	}

	b.Flow = append(b.Flow, httppipeline.Flow{Filter: backendName})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind": proxy.Kind,
		"name": backendName,
		"mainPool": &proxy.PoolSpec{
			Servers:     mainServers,
			LoadBalance: lb,
		},
	})

	return b
}

func (ings *ingressServer) httpServerYAML(httpServer *httpserver.Spec) string {
	ingressHTTPServerFormat := `
kind: HTTPServer
name: %s
http3: %t
port: %d
keepAlive: %t
keepAliveTimeout: %s
https: %t
certBase64: %s
keyBase64: %s
maxConnections: %d
`
	return fmt.Sprintf(ingressHTTPServerFormat,
		ings.superSpec.Name(), httpServer.HTTP3, httpServer.Port, httpServer.KeepAlive,
		httpServer.KeepAliveTimeout, httpServer.HTTPS, httpServer.CertBase64,
		httpServer.KeyBase64, httpServer.MaxConnections)
}

// Init creates a default ingress HTTPServer.
func (ings *ingressServer) Init() error {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if ings.httpServer != nil {
		return nil
	}
	spec := ings.superSpec.ObjectSpec().(*spec.Admin)

	ings.faasNetworkLayerURL = spec.Knative.NetworkLayerURL
	ings.faasHostSuffix = spec.Knative.HostSuffix
	ings.faasNamespace = spec.Knative.Namespace

	yamlConf := ings.httpServerYAML(spec.HTTPServer)

	superSpec, err := supervisor.NewSpec(string(yamlConf))
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConf, err)
		return err
	}

	ings.httpServerSpec = superSpec
	entity, err := ings.tc.CreateHTTPServerForSpec(ings.namespace, superSpec)
	if err != nil {
		return fmt.Errorf("create http server %s failed: %v", superSpec.Name(), err)
	}
	ings.httpServer = entity
	logger.Infof("FaasController :%s init Ingress ok", superSpec.Name())
	return nil
}

func (ings *ingressServer) updateHTTPServer(spec *httpserver.Spec) error {
	buff, err := yaml.Marshal(spec)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", spec, err)
		return err
	}
	httpServerFormat := ` 
name: %s
kind: HTTPServer
%s
`
	yamlConf := fmt.Sprintf(httpServerFormat, ings.superSpec.Name(), buff)
	ings.httpServerSpec, err = supervisor.NewSpec(yamlConf)
	if err != nil {
		return fmt.Errorf("BUG marshal httpserver failed: %v", err)
	}
	_, err = ings.tc.ApplyHTTPServerForSpec(ings.namespace, ings.httpServerSpec)
	if err != nil {
		return fmt.Errorf("apply http server %s failed: %v", ings.httpServerSpec.Name(), err)
	}
	return nil
}

func (ings *ingressServer) find(pipeline string) int {
	spec := ings.httpServerSpec.ObjectSpec().(*httpserver.Spec)
	index := -1
	for idx, v := range spec.Rules {
		for _, p := range v.Paths {
			if p.Backend == pipeline {
				index = idx
				break
			}
		}
	}
	return index
}

func (ings *ingressServer) add(pipeline string) error {
	spec := ings.httpServerSpec.ObjectSpec().(*httpserver.Spec)
	index := ings.find(pipeline)
	// not backend as function's pipeline
	if index == -1 {
		rule := httpserver.Rule{
			Paths: []httpserver.Path{
				{
					PathPrefix: "/",
					Headers: []*httpserver.Header{
						{
							Key:     ingressFunctionKey,
							Values:  []string{pipeline},
							Backend: pipeline,
						},
					},
					Backend: pipeline,
				},
			},
		}
		spec.Rules = append(spec.Rules, rule)
		if err := ings.updateHTTPServer(spec); err != nil {
			logger.Errorf("update http server failed: %v ", err)
		}
	}
	return nil
}

func (ings *ingressServer) remove(pipeline string) error {
	spec := ings.httpServerSpec.ObjectSpec().(*httpserver.Spec)
	index := ings.find(pipeline)

	if index != -1 {
		spec.Rules = append(spec.Rules[:index], spec.Rules[index+1:]...)
		logger.Errorf("remove %#v,", spec)
		return ings.updateHTTPServer(spec)
	}
	return nil
}

// Put puts pipeline named by faas function's name with a requestAdaptor and proxy
func (ings *ingressServer) Put(funcSpec *spec.Spec) error {
	builder := newPipelineSpecBuilder(funcSpec.Name)
	builder.appendReqAdaptor(funcSpec, ings.faasNamespace, ings.faasHostSuffix)
	builder.appendProxy(ings.faasNetworkLayerURL)

	yamlConfig := builder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return err
	}
	if _, err = ings.tc.CreateHTTPPipelineForSpec(ings.namespace, superSpec); err != nil {
		return fmt.Errorf("create http pipeline %s failed: %v", superSpec.Name(), err)
	}
	ings.add(funcSpec.Name)
	ings.pipelines[funcSpec.Name] = struct{}{}

	return nil
}

// Delete deletes one ingress pipeline according to the function's name.
func (ings *ingressServer) Delete(functionName string) {
	ings.mutex.Lock()
	_, exist := ings.pipelines[functionName]
	if exist {
		delete(ings.pipelines, functionName)
	}
	ings.mutex.Unlock()
	if exist {
		ings.remove(functionName)
	}
}

// Update updates ingress's all pipeline by all functions map. In Easegress scenario,
// this function can add back all function's pipeline in store.
func (ings *ingressServer) Update(allFunctions map[string]*spec.Function) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()
	for _, v := range allFunctions {
		index := ings.find(v.Spec.Name)
		_, exist := ings.pipelines[v.Spec.Name]

		if v.Status.State == spec.ActiveState {
			// need to add rule in HTTPServer or create this pipeline
			// especially in reboot scenario.
			if index == -1 || !exist {
				err := ings.Put(v.Spec)
				if err != nil {
					logger.Errorf("ingress add back local pipeline: %s, failed: %v",
						v.Spec.Name, err)
					continue
				}
			}
		} else {
			// Function not ready, then remove it from HTTPServer's route rule
			if index != -1 {
				ings.remove(v.Spec.Name)
			}
		}
	}
}

// Stop stops one ingress pipeline according to the function's name.
func (ings *ingressServer) Stop(functionName string) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.remove(functionName)
}

// Start starts one ingress pipeline according to the function's name.
func (ings *ingressServer) Start(functionName string) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.add(functionName)
}

// Close closes the Egress HTTPServer and Pipelines
func (ings *ingressServer) Close() {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.tc.DeleteHTTPServer(ings.namespace, ings.httpServer.Spec().Name())
	for name := range ings.pipelines {
		ings.tc.DeleteHTTPPipeline(ings.namespace, name)
	}
}

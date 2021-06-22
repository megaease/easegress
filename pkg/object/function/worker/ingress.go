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
	"net/http"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filter/proxy"
	"github.com/megaease/easegress/pkg/filter/requestadaptor"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/function/spec"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
)

const ingressFunctionKey = "X-FaaS-Func-Name"

type (
	faasPipeline struct {
		active   bool
		pipeline *httppipeline.HTTPPipeline
	}

	// ingressServer manages one/many ingress pipelines and one HTTPServer
	ingressServer struct {
		pipelines  map[string]*faasPipeline //*httppipeline.HTTPPipeline
		httpServer *httpserver.HTTPServer

		super     *supervisor.Supervisor
		superSpec *supervisor.Spec

		faasNetworkLayerURL string
		faasHostSuffix      string
		faasNamespace       string

		mutex sync.RWMutex
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
	return &ingressServer{
		pipelines: make(map[string]*faasPipeline),
		super:     super,
		superSpec: superSpec,
		mutex:     sync.RWMutex{},
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

// Get gets ingressServer itself as the default backend.
// egress server will handle the pipeline routing by itself.
func (ings *ingressServer) Get(name string) (protocol.HTTPHandler, bool) {
	ings.mutex.RLock()
	defer ings.mutex.RUnlock()
	return ings, true
}

func (ings *ingressServer) httpServerSpec(httpServer *httpserver.Spec) string {
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
rules:
  - paths:
    - pathPrefix: /
      backend: accept-all-placeholder`

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

	yamlConf := ings.httpServerSpec(spec.HTTPServer)
	logger.Debugf("http svr spec is %s", yamlConf)

	superSpec, err := supervisor.NewSpec(string(yamlConf))
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConf, err)
		return err
	}

	var httpServer httpserver.HTTPServer
	httpServer.Init(superSpec, ings.super)
	httpServer.InjectMuxMapper(ings)
	ings.httpServer = &httpServer
	return nil
}

// Put puts pipeline named by faas function's name with a requestAdaptor and proxy
func (ings *ingressServer) Put(funcSpec *spec.Spec) error {
	builder := newPipelineSpecBuilder(funcSpec.Name)
	builder.appendReqAdaptor(funcSpec, ings.faasNamespace, ings.faasHostSuffix)
	builder.appendProxy(ings.faasNetworkLayerURL)

	yamlConfig := builder.yamlConfig()
	logger.Debugf("pipeline spec is %s", yamlConfig)

	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return err
	}
	pipeline := &httppipeline.HTTPPipeline{}

	pipeline.Init(superSpec, ings.super)
	ings.pipelines[funcSpec.Name] = &faasPipeline{pipeline: pipeline, active: false}

	return nil
}

// Delete deletes one ingress pipeline according to the function's name.
func (ings *ingressServer) Delete(functionName string) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if p, ok := ings.pipelines[functionName]; ok {
		p.pipeline.Close()
		delete(ings.pipelines, functionName)
	}
}

// Update updates ingress's all pipeline by all functions map. In Easegress scenario,
// this function can add back all function's pipeline in store.
func (ings *ingressServer) Update(allFunctions map[string]*spec.Function) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()
	for _, v := range allFunctions {
		if p, exist := ings.pipelines[v.Spec.Name]; !exist {
			err := ings.Put(v.Spec)
			if err != nil {
				logger.Errorf("ingress add back local pipeline: %s, failed: %v", v.Spec.Name, err)
				continue
			}
		} else {
			if v.Status.State == spec.ActiveState {
				p.active = true
			} else {
				p.active = false
			}
		}
	}
}

// Stop stops one ingress pipeline according to the function's name.
func (ings *ingressServer) Stop(functionName string) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if p, ok := ings.pipelines[functionName]; ok {
		p.active = false
	}
}

// Start starts one ingress pipeline according to the function's name.
func (ings *ingressServer) Start(functionName string) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if p, ok := ings.pipelines[functionName]; ok {
		p.active = true
	}
}

func (ings *ingressServer) get(functionName string) (*faasPipeline, error) {
	ings.mutex.RLock()
	defer ings.mutex.RUnlock()

	if pipeline, exist := ings.pipelines[functionName]; exist {
		return pipeline, nil
	}

	return nil, errFunctionNotFound
}

// Handle handles all egress traffic and route to desired pipeline according
// to the "X-FaaS-Func-name" field in header.
func (ings *ingressServer) Handle(ctx context.HTTPContext) {
	name := ctx.Request().Header().Get(ingressFunctionKey)
	if len(name) == 0 {
		logger.Errorf("handle egress RPC without setting service name in: %s header: %#v",
			ingressFunctionKey, ctx.Request().Header())
		ctx.Response().SetStatusCode(http.StatusNotFound)
		return
	}

	faasPipeline, err := ings.get(name)
	if err != nil {
		if err == errFunctionNotFound {
			logger.Errorf("handle faas ingress unknown function: %s", name)
			ctx.Response().SetStatusCode(http.StatusNotFound)
		} else {
			logger.Errorf("handle faas ingress function: %s get pipeline failed: %v", name, err)
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
		}
		return
	}

	if !faasPipeline.active {
		logger.Errorf("handle faas ingress with inactive pipeline: %s", name)
		ctx.Response().SetStatusCode(http.StatusInternalServerError)
		return
	}

	faasPipeline.pipeline.Handle(ctx)
	logger.Debugf("handle service name:%s finished, status code: %d req: %#v", name, ctx.Response().StatusCode(), ctx.Request())
}

// Close closes the Egress HTTPServer and Pipelines
func (ings *ingressServer) Close() {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.httpServer.Close()
	for _, v := range ings.pipelines {
		v.pipeline.Close()
	}
}

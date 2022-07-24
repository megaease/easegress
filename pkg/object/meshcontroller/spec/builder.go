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

package spec

import (
	"fmt"

	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/filters/meshadaptor"
	"github.com/megaease/easegress/pkg/filters/mock"
	"github.com/megaease/easegress/pkg/filters/proxy"
	"github.com/megaease/easegress/pkg/filters/ratelimiter"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/pipeline"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/yamltool"

	"gopkg.in/yaml.v3"
)

type (
	pipelineSpecBuilder struct {
		Kind string `yaml:"kind"`
		Name string `yaml:"name"`

		mockName           string
		rateLimiterName    string
		circuitBreakerName string
		retryName          string
		meshAdaptorName    string
		proxyName          string

		// NOTE: Can't use *pipeline.Spec here.
		// Reference: https://github.com/go-yaml/yaml/issues/356
		pipeline.Spec `yaml:",inline"`
	}

	proxyParam struct {
		instanceSpecs        []*ServiceInstanceSpec
		canaries             []*ServiceCanary
		lb                   *proxy.LoadBalanceSpec
		cert                 *Certificate
		rootCert             *Certificate
		timeout              string
		retryPolicy          string
		circuitBreakerPolicy string
		failureCodes         []int
	}
)

func newPipelineSpecBuilder(name string) *pipelineSpecBuilder {
	return &pipelineSpecBuilder{
		Kind: pipeline.Kind,
		Name: name,

		mockName:           "mock",
		rateLimiterName:    "rateLimiter",
		circuitBreakerName: "circuitBreaker",
		retryName:          "retry",
		meshAdaptorName:    "meshAdaptor",
		proxyName:          "proxy",

		Spec: pipeline.Spec{},
	}
}

func (b *pipelineSpecBuilder) yamlConfig() string {
	buff, err := yaml.Marshal(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", b, err)
	}
	return string(buff)
}

func (b *pipelineSpecBuilder) appendRateLimiter(rule *ratelimiter.Rule) *pipelineSpecBuilder {
	if rule == nil || len(rule.Policies) == 0 || len(rule.URLs) == 0 {
		return b
	}

	spec := ratelimiter.Spec{
		BaseSpec: filters.BaseSpec{
			MetaSpec: supervisor.MetaSpec{
				Name: b.rateLimiterName,
				Kind: ratelimiter.Kind,
			},
		},
	}

	m, err := yamltool.StructToMap(spec)
	if err != nil {
		logger.Errorf("BUG: convert %#v to map failed: %v", spec, err)
		return b
	}

	b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: b.rateLimiterName})
	b.Filters = append(b.Filters, m)

	return b
}

func (b *pipelineSpecBuilder) appendCircuitBreaker(rule *resilience.CircuitBreakerRule) *pipelineSpecBuilder {
	if rule == nil {
		return b
	}

	spec := resilience.CircuitBreakerPolicy{
		BaseSpec: resilience.BaseSpec{
			MetaSpec: supervisor.MetaSpec{
				Name: b.circuitBreakerName,
				Kind: resilience.CircuitBreakerKind.Name,
			},
		},
		CircuitBreakerRule: *rule,
	}

	m, err := yamltool.StructToMap(spec)
	if err != nil {
		logger.Errorf("BUG: convert %#v to map failed: %v", spec, err)
		return b
	}

	b.Resilience = append(b.Resilience, m)

	return b
}

func (b *pipelineSpecBuilder) appendRetry(rule *resilience.RetryRule) *pipelineSpecBuilder {
	if rule == nil {
		return b
	}

	spec := resilience.RetryPolicy{
		BaseSpec: resilience.BaseSpec{
			MetaSpec: supervisor.MetaSpec{
				Name: b.retryName,
				Kind: resilience.RetryKind.Name,
			},
		},
		RetryRule: *rule,
	}

	m, err := yamltool.StructToMap(spec)
	if err != nil {
		logger.Errorf("BUG: convert %#v to map failed: %v", spec, err)
		return b
	}

	b.Resilience = append(b.Resilience, m)

	return b
}

func (b *pipelineSpecBuilder) appendMock(rules []*mock.Rule) *pipelineSpecBuilder {
	if len(rules) == 0 {
		return b
	}

	spec := &mock.Spec{
		BaseSpec: filters.BaseSpec{
			MetaSpec: supervisor.MetaSpec{
				Name: b.mockName,
				Kind: mock.Kind,
			},
		},
		Rules: rules,
	}

	m, err := yamltool.StructToMap(spec)
	if err != nil {
		logger.Errorf("BUG: convert %#v to map failed: %v", spec, err)
		return b
	}

	b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: b.mockName})
	b.Filters = append(b.Filters, m)

	return b
}

func (b *pipelineSpecBuilder) appendProxyWithCanary(param *proxyParam) *pipelineSpecBuilder {
	if param.lb == nil {
		param.lb = &proxy.LoadBalanceSpec{}
	}

	proxySpec := &proxy.Spec{
		BaseSpec: filters.BaseSpec{
			MetaSpec: supervisor.MetaSpec{
				Name: b.proxyName,
				Kind: proxy.Kind,
			},
		},
	}

	needMTLS := false
	if param.cert != nil && param.rootCert != nil {
		needMTLS = true
	}
	if needMTLS {
		proxySpec.MTLS = &proxy.MTLS{
			CertBase64:     param.cert.CertBase64,
			KeyBase64:      param.cert.KeyBase64,
			RootCertBase64: param.rootCert.CertBase64,
		}
	}

	makeServer := func(instance *ServiceInstanceSpec) *proxy.Server {
		var protocol string
		if needMTLS {
			protocol = "https"
		} else {
			protocol = "http"
		}
		return &proxy.Server{
			URL: fmt.Sprintf("%s://%s:%d", protocol, instance.IP, instance.Port),
		}
	}

	mainPool := &proxy.ServerPoolSpec{
		LoadBalance:          param.lb,
		Timeout:              param.timeout,
		RetryPolicy:          param.retryPolicy,
		CircuitBreakerPolicy: param.circuitBreakerPolicy,
		FailureCodes:         param.failureCodes,
	}
	candidatePools := make([]*proxy.ServerPoolSpec, len(param.canaries))

	for _, instance := range param.instanceSpecs {
		if instance.Status != ServiceStatusUp {
			continue
		}

		server := makeServer(instance)

		isCanary := false
		for i, canary := range param.canaries {
			if !canary.Selector.MatchInstance(instance.ServiceName,
				instance.Labels) {
				continue
			}

			if candidatePools[i] == nil {
				headers := canary.TrafficRules.Clone().Headers
				headers[ServiceCanaryHeaderKey] = &proxy.StringMatcher{
					Exact: canary.Name,
				}
				candidatePools[i] = &proxy.ServerPoolSpec{
					Filter: &proxy.RequestMatcherSpec{
						MatchAllHeaders: true,
						Headers:         headers,
					},
					LoadBalance:          param.lb,
					Timeout:              param.timeout,
					RetryPolicy:          param.retryPolicy,
					CircuitBreakerPolicy: param.circuitBreakerPolicy,
					FailureCodes:         param.failureCodes,
				}
			}

			candidatePools[i].Servers = append(candidatePools[i].Servers, server)

			isCanary = true
		}

		if !isCanary {
			mainPool.Servers = append(mainPool.Servers, server)
		}
	}

	proxySpec.Pools = append(proxySpec.Pools, mainPool)

	for _, candidate := range candidatePools {
		if candidate == nil || len(candidate.Servers) == 0 {
			continue
		}

		proxySpec.Pools = append(proxySpec.Pools, candidate)
	}

	m, err := yamltool.StructToMap(proxySpec)
	if err != nil {
		logger.Errorf("BUG: convert %#v to map failed: %v", proxySpec, err)
		return b
	}

	b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: b.proxyName})
	b.Filters = append(b.Filters, m)

	return b
}

func (b *pipelineSpecBuilder) appendMeshAdaptor(canaries []*ServiceCanary) *pipelineSpecBuilder {
	if len(canaries) == 0 {
		return b
	}

	meshAdaptorSpec := &meshadaptor.Spec{
		BaseSpec: filters.BaseSpec{
			MetaSpec: supervisor.MetaSpec{
				Name: b.meshAdaptorName,
				Kind: meshadaptor.Kind,
			},
		},
	}

	adaptors := make([]*meshadaptor.ServiceCanaryAdaptor, len(canaries))
	for i, canary := range canaries {
		// NOTE: It means that setting `X-Mesh-Service-Canary: canaryName`
		// if `X-Mesh-Service-Canary` does not exist and other headers are matching.
		headers := canary.TrafficRules.Clone().Headers
		headers[ServiceCanaryHeaderKey] = &proxy.StringMatcher{
			Empty: true,
		}
		adaptors[i] = &meshadaptor.ServiceCanaryAdaptor{
			Filter: &proxy.RequestMatcherSpec{
				MatchAllHeaders: true,
				Headers:         headers,
			},
			Header: &httpheader.AdaptSpec{
				Set: map[string]string{
					ServiceCanaryHeaderKey: canary.Name,
				},
			},
		}
	}

	meshAdaptorSpec.ServiceCanaries = adaptors

	m, err := yamltool.StructToMap(meshAdaptorSpec)
	if err != nil {
		logger.Errorf("BUG: convert %#v to map failed: %v", meshAdaptorSpec, err)
		return b
	}

	b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: b.meshAdaptorName})
	b.Filters = append(b.Filters, m)

	return b
}

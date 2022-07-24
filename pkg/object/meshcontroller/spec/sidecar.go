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

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

// SidecarEgressServerName returns egress HTTP server name
func (s *Service) SidecarEgressServerName() string {
	return fmt.Sprintf("sidecar-egress-server-%s", s.Name)
}

// SidecarEgressPipelineName returns egress pipeline name
func (s *Service) SidecarEgressPipelineName() string {
	return fmt.Sprintf("sidecar-egress-pipeline-%s", s.Name)
}

// SidecarIngressHTTPServerName returns the ingress server name
func (s *Service) SidecarIngressHTTPServerName() string {
	return fmt.Sprintf("sidecar-ingress-server-%s", s.Name)
}

// SidecarIngressPipelineName returns the ingress pipeline name
func (s *Service) SidecarIngressPipelineName() string {
	return fmt.Sprintf("sidecar-ingress-pipeline-%s", s.Name)
}

// ApplicationInstanceSpec returns instance spec of application.
func (s *Service) ApplicationInstanceSpec(port uint32) *ServiceInstanceSpec {
	return &ServiceInstanceSpec{
		IP:     s.Sidecar.Address,
		Port:   port,
		Status: ServiceStatusUp,
	}
}

// SidecarEgressHTTPServerSpec returns a spec for egress HTTP server
func (s *Service) SidecarEgressHTTPServerSpec(keepalive bool, timeout string) (*supervisor.Spec, error) {
	egressHTTPServerFormat := `
kind: HTTPServer
name: %s
port: %d
keepAlive: %v
keepAliveTimeout: %s
https: false
`
	if timeout == "" {
		timeout = defaultKeepAliveTimeout
	}
	yamlConfig := fmt.Sprintf(egressHTTPServerFormat,
		s.SidecarEgressServerName(),
		s.Sidecar.EgressPort,
		keepalive,
		timeout)

	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", err)
		return nil, err
	}

	return superSpec, nil
}

// SidecarEgressPipelineSpec returns a spec for sidecar egress pipeline
func (s *Service) SidecarEgressPipelineSpec(instanceSpecs []*ServiceInstanceSpec,
	canaries []*ServiceCanary, appCert, rootCert *Certificate,
) (*supervisor.Spec, error) {
	if len(instanceSpecs) == 0 {
		return nil, fmt.Errorf("no instance")
	}

	pipelineSpecBuilder := newPipelineSpecBuilder(s.SidecarEgressPipelineName())

	pipelineSpecBuilder.appendMeshAdaptor(canaries)

	if s.Mock != nil && s.Mock.Enabled {
		pipelineSpecBuilder.appendMock(s.Mock.Rules)
	}

	var timeout string
	var retryPolicy string
	var circuitBreakerPolicy string
	var failureCodes []int
	if s.Resilience != nil {
		pipelineSpecBuilder.appendRetry(s.Resilience.Retry)
		pipelineSpecBuilder.appendCircuitBreaker(s.Resilience.CircuitBreaker)
		if s.Resilience.TimeLimiter != nil {
			timeout = s.Resilience.TimeLimiter.Timeout
		}
		if s.Resilience.Retry != nil {
			retryPolicy = pipelineSpecBuilder.retryName
		}
		if s.Resilience.CircuitBreaker != nil {
			circuitBreakerPolicy = pipelineSpecBuilder.circuitBreakerName
		}

		failureCodes = s.Resilience.FailureCodes
	}

	pipelineSpecBuilder.appendProxyWithCanary(&proxyParam{
		instanceSpecs:        instanceSpecs,
		canaries:             canaries,
		lb:                   s.LoadBalance,
		cert:                 appCert,
		rootCert:             rootCert,
		timeout:              timeout,
		retryPolicy:          retryPolicy,
		circuitBreakerPolicy: circuitBreakerPolicy,
		failureCodes:         failureCodes,
	})

	yamlConfig := pipelineSpecBuilder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return superSpec, nil
}

// SidecarIngressHTTPServerSpec generates a spec for sidecar ingress HTTP server
func (s *Service) SidecarIngressHTTPServerSpec(keepalive bool, timeout string,
	cert, rootCert *Certificate,
) (*supervisor.Spec, error) {
	ingressHTTPServerFormat := `
kind: HTTPServer
name: %s
port: %d
keepAlive: %v
keepAliveTimeout: %s
https: %s
certBase64: %s
keyBase64: %s
caCertBase64: %s
rules:
  - paths:
    - pathPrefix: /
      backend: %s`

	name := s.SidecarIngressHTTPServerName()
	pipelineName := s.SidecarIngressPipelineName()
	certBase64, keyBase64, rootCertBaser64, needHTTPS := "", "", "", "false"
	if cert != nil && rootCert != nil {
		certBase64 = cert.CertBase64
		keyBase64 = cert.KeyBase64
		rootCertBaser64 = rootCert.CertBase64
		needHTTPS = "true"
	}
	if timeout == "" {
		timeout = defaultKeepAliveTimeout
	}
	yamlConfig := fmt.Sprintf(ingressHTTPServerFormat, name,
		s.Sidecar.IngressPort, keepalive, timeout, needHTTPS,
		certBase64, keyBase64, rootCertBaser64, pipelineName)

	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return superSpec, nil
}

// SidecarIngressPipelineSpec returns a spec for sidecar ingress pipeline
func (s *Service) SidecarIngressPipelineSpec(applicationPort uint32) (*supervisor.Spec, error) {
	pipelineSpecBuilder := newPipelineSpecBuilder(s.SidecarIngressPipelineName())

	var timeout string
	if s.Resilience != nil {
		pipelineSpecBuilder.appendRateLimiter(s.Resilience.RateLimiter)
		if s.Resilience.TimeLimiter != nil {
			timeout = s.Resilience.TimeLimiter.Timeout
		}
	}

	pipelineSpecBuilder.appendProxyWithCanary(&proxyParam{
		instanceSpecs: []*ServiceInstanceSpec{s.ApplicationInstanceSpec(applicationPort)},
		lb:            s.LoadBalance,
		timeout:       timeout,
	})

	yamlConfig := pipelineSpecBuilder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return superSpec, nil
}

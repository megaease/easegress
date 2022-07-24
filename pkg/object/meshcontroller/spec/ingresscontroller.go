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
	"bytes"
	"fmt"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

// IngressControllerServerName returns the server name of ingress controller.
const IngressControllerServerName = "ingresscontroller-server"

// IngressControllerPipelineName returns the pipeline name of ingress controller.
func (s *Service) IngressControllerPipelineName() string {
	return fmt.Sprintf("ingresscontroller-pipeline-%s", s.Name)
}

// BackendName returns backend service name
func (s *Service) BackendName() string {
	return s.Name
}

// IngressControllerHTTPServerSpec generates HTTP server spec for ingress.
// as ingress does not belong to a service, it is not a method of 'Service'
func IngressControllerHTTPServerSpec(port int, rules []*IngressRule) (*supervisor.Spec, error) {
	const specFmt = `
kind: HTTPServer
name: %s
port: %d
keepAlive: true
https: false
rules:`

	const ruleFmt = `
  - host: %s
    paths:`

	const pathFmt = `
      - pathRegexp: %s
        rewriteTarget: %s
        backend: %s`

	buf := bytes.Buffer{}

	str := fmt.Sprintf(specFmt, IngressControllerServerName, port)
	buf.WriteString(str)

	for _, r := range rules {
		str = fmt.Sprintf(ruleFmt, r.Host)
		buf.WriteString(str)
		for j := range r.Paths {
			p := r.Paths[j]
			str = fmt.Sprintf(pathFmt, p.Path, p.RewriteTarget, p.Backend)
			buf.WriteString(str)
		}
	}

	yamlConfig := buf.String()
	spec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return spec, nil
}

// IngressControllerPipelineSpec generates a spec for ingress controller pipeline spec.
func (s *Service) IngressControllerPipelineSpec(instanceSpecs []*ServiceInstanceSpec,
	canaries []*ServiceCanary, cert, rootCert *Certificate,
) (*supervisor.Spec, error) {
	pipelineSpecBuilder := newPipelineSpecBuilder(s.IngressControllerPipelineName())

	pipelineSpecBuilder.appendMeshAdaptor(canaries)
	pipelineSpecBuilder.appendProxyWithCanary(&proxyParam{
		instanceSpecs: instanceSpecs,
		canaries:      canaries,
		lb:            s.LoadBalance,
		cert:          cert,
		rootCert:      rootCert,
	})

	yamlConfig := pipelineSpecBuilder.yamlConfig()
	superSpec, err := supervisor.NewSpec(yamlConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", yamlConfig, err)
		return nil, err
	}

	return superSpec, nil
}

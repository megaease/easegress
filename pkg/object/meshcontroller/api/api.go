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

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/v"
)

const (
	// MeshPrefix is the mesh prefix.
	MeshPrefix = "/mesh"

	// MeshTenantPrefix is the mesh tenant prefix.
	MeshTenantPrefix = "/mesh/tenants"

	// MeshTenantPath is the mesh tenant path.
	MeshTenantPath = "/mesh/tenants/{tenantName}"

	// MeshIngressPrefix is the mesh ingress prefix.
	MeshIngressPrefix = "/mesh/ingresses"

	// MeshIngressPath is the mesh ingress path.
	MeshIngressPath = "/mesh/ingresses/{ingressName}"

	// MeshServicePrefix is mesh service prefix.
	MeshServicePrefix = "/mesh/services"

	// MeshServicePath is the mesh service path.
	MeshServicePath = "/mesh/services/{serviceName}"

	// OldMeshServiceCanaryPath is the mesh service canary path.
	OldMeshServiceCanaryPath = "/mesh/services/{serviceName}/canary"

	// MeshServiceMockPath is the mesh service mock path.
	MeshServiceMockPath = "/mesh/services/{serviceName}/mock"

	// MeshServiceResiliencePath is the mesh service resilience path.
	MeshServiceResiliencePath = "/mesh/services/{serviceName}/resilience"

	// MeshServiceLoadBalancePath is the mesh service load balance path.
	MeshServiceLoadBalancePath = "/mesh/services/{serviceName}/loadbalance"

	// MeshServiceOutputServerPath is the mesh service output server path.
	MeshServiceOutputServerPath = "/mesh/services/{serviceName}/outputserver"

	// MeshServiceTracingsPath is the mesh service tracings path.
	MeshServiceTracingsPath = "/mesh/services/{serviceName}/tracings"

	// MeshServiceMetricsPath is the mesh service metrics path.
	MeshServiceMetricsPath = "/mesh/services/{serviceName}/metrics"

	// MeshServiceInstancePrefix is the mesh service prefix.
	MeshServiceInstancePrefix = "/mesh/serviceinstances"

	// MeshServiceInstancePath is the mesh service path.
	MeshServiceInstancePath = "/mesh/serviceinstances/{serviceName}/{instanceID}"

	// MeshHTTPRouteGroupPrefix is the mesh HTTP route groups prefix.
	MeshHTTPRouteGroupPrefix = "/mesh/httproutegroups"

	// MeshHTTPRouteGroupPath is the mesh HTTP route groups path.
	MeshHTTPRouteGroupPath = "/mesh/httproutegroups/{name}"

	// MeshTrafficTargetPrefix is the mesh traffic target prefix.
	MeshTrafficTargetPrefix = "/mesh/traffictargets"

	// MeshTrafficTargetPath is the mesh traffic target path.
	MeshTrafficTargetPath = "/mesh/traffictargets/{name}"

	// MeshCustomResourceKindPrefix is the mesh custom resource kind prefix.
	MeshCustomResourceKindPrefix = "/mesh/customresourcekinds"

	// MeshCustomResourceKind is the mesh custom resource kind
	MeshCustomResourceKind = "/mesh/customresourcekinds/{name}"

	// MeshAllCustomResourcePrefix is the mesh custom resource prefix
	MeshAllCustomResourcePrefix = "/mesh/customresources"

	// MeshCustomResourcePrefix is the mesh custom resource prefix of a specified kind
	MeshCustomResourcePrefix = "/mesh/customresources/{kind}"

	// MeshCustomResource is the mesh custom resource of a specified kind
	MeshCustomResource = "/mesh/customresources/{kind}/{name}"

	// MeshWatchCustomResource is the path to watch custom resources of a specified kind
	MeshWatchCustomResource = "/mesh/watchcustomresources/{kind}"

	// MeshServiceCanaryPrefix is the service canary prefix.
	MeshServiceCanaryPrefix = "/mesh/servicecanaries"

	// MeshServiceCanaryPath is the service canary path.
	MeshServiceCanaryPath = "/mesh/servicecanaries/{serviceCanaryName}"
)

type (
	// API is the struct with the service
	API struct {
		service *service.Service
	}
)

const apiGroupName = "mesh_admin"

// New creates a API
func New(superSpec *supervisor.Spec) *API {
	api := &API{
		service: service.New(superSpec),
	}

	api.registerAPIs()

	return api
}

// Close unregisters a API
func (a *API) Close() {
	api.UnregisterAPIs(apiGroupName)
}

func (a *API) registerAPIs() {
	group := &api.Group{
		Group: apiGroupName,
		Entries: []*api.Entry{
			{Path: MeshTenantPrefix, Method: "GET", Handler: a.listTenants},
			{Path: MeshTenantPrefix, Method: "POST", Handler: a.createTenant},
			{Path: MeshTenantPath, Method: "GET", Handler: a.getTenant},
			{Path: MeshTenantPath, Method: "PUT", Handler: a.updateTenant},
			{Path: MeshTenantPath, Method: "DELETE", Handler: a.deleteTenant},
			{Path: MeshIngressPrefix, Method: "GET", Handler: a.listIngresses},
			{Path: MeshIngressPrefix, Method: "POST", Handler: a.createIngress},
			{Path: MeshIngressPath, Method: "GET", Handler: a.getIngress},
			{Path: MeshIngressPath, Method: "PUT", Handler: a.updateIngress},
			{Path: MeshIngressPath, Method: "DELETE", Handler: a.deleteIngress},
			{Path: MeshServicePrefix, Method: "GET", Handler: a.listServices},
			{Path: MeshServicePrefix, Method: "POST", Handler: a.createService},
			{Path: MeshServicePath, Method: "GET", Handler: a.getService},
			{Path: MeshServicePath, Method: "PUT", Handler: a.updateService},
			{Path: MeshServicePath, Method: "DELETE", Handler: a.deleteService},

			// TODO: API to get instances of one service.

			{Path: MeshServiceInstancePrefix, Method: "GET", Handler: a.listServiceInstanceSpecs},
			{Path: MeshServiceInstancePath, Method: "GET", Handler: a.getServiceInstanceSpec},
			{Path: MeshServiceInstancePath, Method: "DELETE", Handler: a.offlineServiceInstance},

			{Path: MeshServiceMockPath, Method: "POST", Handler: a.createPartOfService(mockMeta)},
			{Path: MeshServiceMockPath, Method: "GET", Handler: a.getPartOfService(mockMeta)},
			{Path: MeshServiceMockPath, Method: "PUT", Handler: a.updatePartOfService(mockMeta)},
			{Path: MeshServiceMockPath, Method: "DELETE", Handler: a.deletePartOfService(mockMeta)},

			{Path: MeshServiceResiliencePath, Method: "POST", Handler: a.createPartOfService(resilienceMeta)},
			{Path: MeshServiceResiliencePath, Method: "GET", Handler: a.getPartOfService(resilienceMeta)},
			{Path: MeshServiceResiliencePath, Method: "PUT", Handler: a.updatePartOfService(resilienceMeta)},
			{Path: MeshServiceResiliencePath, Method: "DELETE", Handler: a.deletePartOfService(resilienceMeta)},

			{Path: MeshServiceLoadBalancePath, Method: "POST", Handler: a.createPartOfService(loadBalanceMeta)},
			{Path: MeshServiceLoadBalancePath, Method: "GET", Handler: a.getPartOfService(loadBalanceMeta)},
			{Path: MeshServiceLoadBalancePath, Method: "PUT", Handler: a.updatePartOfService(loadBalanceMeta)},
			{Path: MeshServiceLoadBalancePath, Method: "DELETE", Handler: a.deletePartOfService(loadBalanceMeta)},

			{Path: MeshServiceOutputServerPath, Method: "POST", Handler: a.createPartOfService(outputServerMeta)},
			{Path: MeshServiceOutputServerPath, Method: "GET", Handler: a.getPartOfService(outputServerMeta)},
			{Path: MeshServiceOutputServerPath, Method: "PUT", Handler: a.updatePartOfService(outputServerMeta)},
			{Path: MeshServiceOutputServerPath, Method: "DELETE", Handler: a.deletePartOfService(outputServerMeta)},

			{Path: MeshServiceTracingsPath, Method: "POST", Handler: a.createPartOfService(tracingsMeta)},
			{Path: MeshServiceTracingsPath, Method: "GET", Handler: a.getPartOfService(tracingsMeta)},
			{Path: MeshServiceTracingsPath, Method: "PUT", Handler: a.updatePartOfService(tracingsMeta)},
			{Path: MeshServiceTracingsPath, Method: "DELETE", Handler: a.deletePartOfService(tracingsMeta)},

			{Path: MeshServiceMetricsPath, Method: "POST", Handler: a.createPartOfService(metricsMeta)},
			{Path: MeshServiceMetricsPath, Method: "GET", Handler: a.getPartOfService(metricsMeta)},
			{Path: MeshServiceMetricsPath, Method: "PUT", Handler: a.updatePartOfService(metricsMeta)},
			{Path: MeshServiceMetricsPath, Method: "DELETE", Handler: a.deletePartOfService(metricsMeta)},

			{Path: MeshHTTPRouteGroupPrefix, Method: "GET", Handler: a.listHTTPRouteGroups},
			{Path: MeshHTTPRouteGroupPrefix, Method: "POST", Handler: a.createHTTPRouteGroup},
			{Path: MeshHTTPRouteGroupPath, Method: "GET", Handler: a.getHTTPRouteGroup},
			{Path: MeshHTTPRouteGroupPath, Method: "PUT", Handler: a.updateHTTPRouteGroup},
			{Path: MeshHTTPRouteGroupPath, Method: "DELETE", Handler: a.deleteHTTPRouteGroup},

			{Path: MeshTrafficTargetPrefix, Method: "GET", Handler: a.listTrafficTargets},
			{Path: MeshTrafficTargetPrefix, Method: "POST", Handler: a.createTrafficTarget},
			{Path: MeshTrafficTargetPath, Method: "GET", Handler: a.getTrafficTarget},
			{Path: MeshTrafficTargetPath, Method: "PUT", Handler: a.updateTrafficTarget},
			{Path: MeshTrafficTargetPath, Method: "DELETE", Handler: a.deleteTrafficTarget},

			{Path: MeshCustomResourceKindPrefix, Method: "GET", Handler: a.listCustomResourceKinds},
			{Path: MeshCustomResourceKindPrefix, Method: "POST", Handler: a.createCustomResourceKind},
			{Path: MeshCustomResourceKind, Method: "GET", Handler: a.getCustomResourceKind},
			{Path: MeshCustomResourceKindPrefix, Method: "PUT", Handler: a.updateCustomResourceKind},
			{Path: MeshCustomResourceKind, Method: "DELETE", Handler: a.deleteCustomResourceKind},

			{Path: MeshAllCustomResourcePrefix, Method: "GET", Handler: a.listAllCustomResources},
			{Path: MeshCustomResourcePrefix, Method: "GET", Handler: a.listCustomResources},
			{Path: MeshAllCustomResourcePrefix, Method: "POST", Handler: a.createCustomResource},
			{Path: MeshCustomResource, Method: "GET", Handler: a.getCustomResource},
			{Path: MeshAllCustomResourcePrefix, Method: "PUT", Handler: a.updateCustomResource},
			{Path: MeshCustomResource, Method: "DELETE", Handler: a.deleteCustomResource},

			{Path: MeshWatchCustomResource, Method: "GET", Handler: a.watchCustomResources},

			{Path: MeshServiceCanaryPrefix, Method: "GET", Handler: a.listServiceCanaries},
			{Path: MeshServiceCanaryPrefix, Method: "POST", Handler: a.createServiceCanary},
			{Path: MeshServiceCanaryPath, Method: "GET", Handler: a.getServiceCanary},
			{Path: MeshServiceCanaryPath, Method: "PUT", Handler: a.updateServiceCanary},
			{Path: MeshServiceCanaryPath, Method: "DELETE", Handler: a.deleteServiceCanary},
		},
	}

	api.RegisterAPIs(group)
}

func (a *API) convertSpecToPB(spec interface{}, pbSpec interface{}) error {
	buf, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal %#v to json failed: %v", spec, err)
	}

	err = json.Unmarshal(buf, pbSpec)
	if err != nil {
		return fmt.Errorf("unmarshal from json: %s failed: %v", string(buf), err)
	}

	return nil
}

func (a *API) convertPBToSpec(pbSpec interface{}, spec interface{}) error {
	buff, err := json.Marshal(pbSpec)
	if err != nil {
		return fmt.Errorf("marshal %#v to json: %v", pbSpec, err)
	}

	err = json.Unmarshal(buff, spec)
	if err != nil {
		return fmt.Errorf("unmarshal %#v to spec: %v", spec, err)
	}

	return nil
}

func (a *API) readAPISpec(r *http.Request, pbSpec interface{}, spec interface{}) error {
	// TODO: Use default spec and validate it.

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body failed: %v", err)
	}

	err = json.Unmarshal(body, pbSpec)
	if err != nil {
		return fmt.Errorf("unmarshal %s to pb spec %#v failed: %v", string(body), pbSpec, err)
	}

	err = a.convertPBToSpec(pbSpec, spec)
	if err != nil {
		return err
	}

	vr := v.Validate(spec)
	if !vr.Valid() {
		return fmt.Errorf("validate failed:\n%s", vr)
	}

	return nil
}

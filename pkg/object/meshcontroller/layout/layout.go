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

package layout

import (
	"fmt"
)

const (
	serviceSpecPrefix = "/mesh/service-spec/"
	serviceSpec       = "/mesh/service-spec/%s" // +serviceName

	allServiceInstanceSpecPrefix   = "/mesh/service-instances/spec/"
	allServiceInstanceStatusPrefix = "/mesh/service-instances/status/"
	serviceInstanceSpecPrefix      = "/mesh/service-instances/spec/%s/"     // +serviceName
	serviceInstanceStatusPrefix    = "/mesh/service-instances/status/%s/"   // +serviceName
	serviceInstanceSpec            = "/mesh/service-instances/spec/%s/%s"   // +serviceName +instanceID
	serviceInstanceStatus          = "/mesh/service-instances/status/%s/%s" // +serviceName +instanceID

	allIngressControllerInstanceSpecPrefix = "/mesh/ingresscontroller/spec/"
	ingressControllerInstanceSpecKey       = "/mesh/ingresscontroller/spec/%s" //+instanceID

	tenant       = "/mesh/tenants/%s" // +tenantName
	tenantPrefix = "/mesh/tenants/"

	ingress       = "/mesh/ingress/%s" // + ingressName
	ingressPrefix = "/mesh/ingress/"

	serviceInstanceCert                    = "/mesh/cert/service-cert/%s/%s" // +serviceName +instanceID
	allServiceCertPrefix                   = "/mesh/cert/service-cert/"
	rootCert                               = "/mesh/cert/root-cert"
	ingressControllerInstanceCertKey       = "/mesh/cert/ingress-controller-cert/%s"
	allIngressControllerInstanceCertPrefix = "/mesh/cert/ingress-controller-cert/"

	httpRouteGroup       = "/mesh/http-route-groups/%s" // + httpRouteGroupName
	httpRouteGroupPrefix = "/mesh/http-route-groups/"
	trafficTarget        = "/mesh/traffic-targets/%s" // + trafficTargetName
	trafficTargetPrefix  = "/mesh/traffic-targets/"

	customResourceKindPrefix = "/mesh/custom-resource-kinds/"
	customResourceKind       = "/mesh/custom-resource-kinds/%s" // +kind
	allCustomResourcePrefix  = "/mesh/custom-resources/"
	customResourcePrefix     = "/mesh/custom-resources/%s/"   // +kind
	customResource           = "/mesh/custom-resources/%s/%s" // +kind +name

	serviceCanaryPrefix = "/mesh/service-canary/"
	serviceCanary       = "/mesh/service-canary/%s"
)

// ServiceSpecPrefix returns the prefix of service.
func ServiceSpecPrefix() string {
	return serviceSpecPrefix
}

// ServiceSpecKey returns the key of service spec.
func ServiceSpecKey(serviceName string) string {
	return fmt.Sprintf(serviceSpec, serviceName)
}

// ServiceInstanceSpecKey returns the key of service instance spec.
func ServiceInstanceSpecKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceSpec, serviceName, instanceID)
}

// ServiceInstanceStatusKey returns the key of service instance status.
func ServiceInstanceStatusKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceStatus, serviceName, instanceID)
}

// ServiceInstanceSpecPrefix returns the prefix of service instance specs.
func ServiceInstanceSpecPrefix(serviceName string) string {
	return fmt.Sprintf(serviceInstanceSpecPrefix, serviceName)
}

// ServiceInstanceStatusPrefix returns the prefix of service instance statuses.
func ServiceInstanceStatusPrefix(serviceName string) string {
	return fmt.Sprintf(serviceInstanceStatusPrefix, serviceName)
}

// AllServiceInstanceSpecPrefix returns the prefix of all service instance specs.
func AllServiceInstanceSpecPrefix() string {
	return allServiceInstanceSpecPrefix
}

// AllServiceInstanceStatusPrefix returns the prefix of all service instance statuses.
func AllServiceInstanceStatusPrefix() string {
	return allServiceInstanceStatusPrefix
}

// TenantSpecKey returns the key of tenant spec.
func TenantSpecKey(t string) string {
	return fmt.Sprintf(tenant, t)
}

// TenantPrefix returns the prefix of tenant.
func TenantPrefix() string {
	return tenantPrefix
}

// IngressSpecKey returns the key of ingress spec.
func IngressSpecKey(t string) string {
	return fmt.Sprintf(ingress, t)
}

// IngressPrefix returns the prefix of ingress.
func IngressPrefix() string {
	return ingressPrefix
}

// HTTPRouteGroupKey returns the key of HTTP route group spec.
func HTTPRouteGroupKey(t string) string {
	return fmt.Sprintf(httpRouteGroup, t)
}

// HTTPRouteGroupPrefix returns the prefix of HTTP route groups.
func HTTPRouteGroupPrefix() string {
	return httpRouteGroupPrefix
}

// TrafficTargetKey returns the key of traffic target spec.
func TrafficTargetKey(t string) string {
	return fmt.Sprintf(trafficTarget, t)
}

// TrafficTargetPrefix returns the prefix of traffic targets.
func TrafficTargetPrefix() string {
	return trafficTargetPrefix
}

// CustomResourceKindPrefix returns the prefix of custom object kinds.
func CustomResourceKindPrefix() string {
	return customResourceKindPrefix
}

// CustomResourceKindKey returns the key of specified custom object kind.
func CustomResourceKindKey(kind string) string {
	return fmt.Sprintf(customResourceKind, kind)
}

// AllCustomResourcePrefix returns the prefix of custom objects.
func AllCustomResourcePrefix() string {
	return allCustomResourcePrefix
}

// CustomResourcePrefix returns the prefix of custom objects.
func CustomResourcePrefix(kind string) string {
	return fmt.Sprintf(customResourcePrefix, kind)
}

// CustomResourceKey returns the key of specified custom object.
func CustomResourceKey(kind, name string) string {
	return fmt.Sprintf(customResource, kind, name)
}

// ServiceInstanceCertKey returns the key of specified service's cert.
func ServiceInstanceCertKey(serviceName, instanceID string) string {
	return fmt.Sprintf(serviceInstanceCert, serviceName, instanceID)
}

// AllServiceCertPrefix returns the prefix of all service's cert.
func AllServiceCertPrefix() string {
	return allServiceCertPrefix
}

// RootCertKey returns the root cert key.
func RootCertKey() string {
	return rootCert
}

// IngressControllerInstanceCertKey returns one ingresscontroller's instance key.
func IngressControllerInstanceCertKey(instanceID string) string {
	return fmt.Sprintf(ingressControllerInstanceCertKey, instanceID)
}

// IngressControllerInstanceSpecKey returns one ingresscontroller's instance key.
func IngressControllerInstanceSpecKey(instanceID string) string {
	return fmt.Sprintf(ingressControllerInstanceSpecKey, instanceID)
}

// AllIngressControllerInstanceSpecPrefix returns all instances specs prefix.
func AllIngressControllerInstanceSpecPrefix() string {
	return allIngressControllerInstanceSpecPrefix
}

// AllIngressControllerInstanceCertPrefix returns all instances specs prefix.
func AllIngressControllerInstanceCertPrefix() string {
	return allIngressControllerInstanceCertPrefix
}

// ServiceCanaryPrefix returns the prefix of service canaries.
func ServiceCanaryPrefix() string {
	return serviceCanaryPrefix
}

// ServiceCanaryKey returns the key of service canary.
func ServiceCanaryKey(serviceCanaryName string) string {
	return fmt.Sprintf(serviceCanary, serviceCanaryName)
}

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
	"time"

	"github.com/megaease/easegress/pkg/cluster/customdata"
	"github.com/megaease/easegress/pkg/filters/mock"
	"github.com/megaease/easegress/pkg/filters/proxy"
	"github.com/megaease/easegress/pkg/filters/ratelimiter"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/util/urlrule"
)

const (
	// RegistryTypeConsul is the consul registry type.
	RegistryTypeConsul = "consul"
	// RegistryTypeEureka is the eureka registry type.
	RegistryTypeEureka = "eureka"
	// RegistryTypeNacos is the eureka registry type.
	RegistryTypeNacos = "nacos"

	// GlobalTenant is the reserved name of the system scope tenant,
	// its services can be accessible in mesh wide.
	GlobalTenant = "global"

	// ServiceStatusUp indicates this service instance can accept ingress traffic
	ServiceStatusUp = "UP"

	// ServiceStatusOutOfService indicates this service instance can't accept ingress traffic
	ServiceStatusOutOfService = "OUT_OF_SERVICE"

	// WorkerAPIPort is the default port for worker's API server
	WorkerAPIPort = 13009

	// IngressPort is the default port for ingress controller
	IngressPort = 13010

	// HeartbeatInterval is the default heartbeat interval for checking service heartbeat
	HeartbeatInterval = "5s"

	// SecurityLevelPermissive is the level for not enabling mTLS.
	SecurityLevelPermissive = "permissive"

	// SecurityLevelStrict is the level for enabling mTLS.
	SecurityLevelStrict = "strict"

	// DefaultCommonName is the name of root ca cert.
	DefaultCommonName = "mesh-root-ca"

	// CertProviderSelfSign is the in-memory, self-sign cert provider.
	CertProviderSelfSign = "selfSign"

	// IngressControllerName is the name of easemesh ingress controller.
	IngressControllerName = "ingresscontroller"

	// from k8s pod's env value

	// PodEnvHostname is the name of the pod in environment variable.
	PodEnvHostname = "HOSTNAME"

	// PodEnvApplicationIP is the IP of the pod in environment variable.
	PodEnvApplicationIP = "APPLICATION_IP"

	// ServiceCanaryHeaderKey is the http header key of service canary.
	ServiceCanaryHeaderKey = "X-Mesh-Service-Canary"

	defaultKeepAliveTimeout = "60s"
)

var (
	// ErrParamNotMatch means RESTful request URL's object name or other fields are not matched in this request's body
	ErrParamNotMatch = fmt.Errorf("param in url and body's spec not matched")
	// ErrAlreadyRegistered indicates this instance has already been registered
	ErrAlreadyRegistered = fmt.Errorf("service already registered")
	// ErrNoRegisteredYet indicates this instance haven't registered successfully yet
	ErrNoRegisteredYet = fmt.Errorf("service not registered yet")
	// ErrServiceNotFound indicates could find target service in its tenant or in global tenant
	ErrServiceNotFound = fmt.Errorf("can't find service in its tenant or in global tenant")
	// ErrServiceNotavailable indicates could find target service's available instances.
	ErrServiceNotavailable = fmt.Errorf("can't find service available instances")
)

type (
	// Admin is the spec of MeshController.
	Admin struct {
		// HeartbeatInterval is the interval for one service instance reporting its heartbeat.
		HeartbeatInterval string `yaml:"heartbeatInterval" jsonschema:"required,format=duration"`

		// RegistryTime indicates which protocol the registry center accepts.
		RegistryType string `yaml:"registryType" jsonschema:"required"`

		// APIPort is the port for worker's API server
		APIPort int `yaml:"apiPort" jsonschema:"required"`

		// IngressPort is the port for http server in mesh ingress
		IngressPort int `yaml:"ingressPort" jsonschema:"required"`

		ExternalServiceRegistry string `yaml:"externalServiceRegistry" jsonschema:"omitempty"`

		CleanExternalRegistry bool `yaml:"cleanExternalRegistry"`

		Security *Security `yaml:"security" jsonschema:"omitempty"`

		// Sidecar injection relevant config.
		ImageRegistryURL          string `yaml:"imageRegistryURL" jsonschema:"omitempty"`
		ImagePullPolicy           string `yaml:"imagePullPolicy" jsonschema:"omitempty"`
		SidecarImageName          string `yaml:"sidecarImageName" jsonschema:"omitempty"`
		AgentInitializerImageName string `yaml:"agentInitializerImageName" jsonschema:"omitempty"`
		Log4jConfigName           string `yaml:"log4jConfigName" jsonschema:"omitempty"`

		MonitorMTLS *MonitorMTLS `yaml:"monitorMTLS" jsonschema:"omitempty"`
		WorkerSpec  WorkerSpec   `yaml:"workerSpec" jsonschema:"omitempty"`
	}

	// WorkerSpec is the spec of worker
	WorkerSpec struct {
		Ingress IngressServerSpec `yaml:"ingress" jsonschema:"omitempty"`
		Egress  EgressServerSpec  `yaml:"egress" jsonschema:"omitempty"`
	}

	// IngressServerSpec is the spec of ingress httpserver in worker
	IngressServerSpec struct {
		KeepAlive        bool   `yaml:"keepAlive" jsonschema:"omitempty"`
		KeepAliveTimeout string `yaml:"keepAliveTimeout" jsonschema:"omitempty,format=duration"`
	}

	// EgressServerSpec is the spec of egress httpserver in worker
	EgressServerSpec struct {
		KeepAlive        bool   `yaml:"keepAlive" jsonschema:"omitempty"`
		KeepAliveTimeout string `yaml:"keepAliveTimeout" jsonschema:"omitempty,format=duration"`
	}

	// MonitorMTLS is the spec of mTLS specification of monitor.
	MonitorMTLS struct {
		Enabled  bool   `yaml:"enabled" jsonschema:"required"`
		URL      string `yaml:"url" jsonschema:"required"`
		Username string `yaml:"username" jsonschema:"required"`
		Password string `yaml:"password" jsonschema:"required"`

		ReporterAppendType string         `yaml:"reporterAppendType"`
		CaCertBase64       string         `yaml:"caCertBase64" jsonschema:"required,format=base64"`
		Certs              []*MonitorCert `yaml:"certs" jsonschema:"required"`
	}

	// MonitorCert is the spec for single pack of mTLS.
	MonitorCert struct {
		CertBase64 string   `yaml:"certBase64" jsonschema:"required,format=base64"`
		KeyBase64  string   `yaml:"keyBase64" jsonschema:"required,format=base64"`
		Services   []string `yaml:"services" jsonschema:"required"`
	}

	// Security is the spec for mesh-wide security.
	Security struct {
		MTLSMode     string `yaml:"mtlsMode" jsonschema:"required"`
		CertProvider string `yaml:"certProvider" jsonschema:"required"`

		RootCertTTL string `yaml:"rootCertTTL" jsonschema:"required,format=duration"`
		AppCertTTL  string `yaml:"appCertTTL" jsonschema:"required,format=duration"`
	}

	// Service contains the information of service.
	Service struct {
		// Empty means mesh registry itself.
		RegistryName   string `yaml:"registryName" jsonschema:"omitempty"`
		Name           string `yaml:"name" jsonschema:"required"`
		RegisterTenant string `yaml:"registerTenant" jsonschema:"required"`

		Sidecar       *Sidecar       `yaml:"sidecar" jsonschema:"required"`
		Mock          *Mock          `yaml:"mock" jsonschema:"omitempty"`
		Resilience    *Resilience    `yaml:"resilience" jsonschema:"omitempty"`
		LoadBalance   *LoadBalance   `yaml:"loadBalance" jsonschema:"omitempty"`
		Observability *Observability `yaml:"observability" jsonschema:"omitempty"`
	}

	// Mock is the spec of configured and static API responses for this service.
	Mock struct {
		// Enable is the mocking switch for this service.
		Enabled bool `yaml:"enabled" jsonschema:"required"`

		// Rules are the mocking matching and responding configurations.
		Rules []*mock.Rule `yaml:"rules" jsonschema:"omitempty"`
	}

	// Resilience is the spec of service resilience.
	Resilience struct {
		RateLimiter    *ratelimiter.Rule              `yaml:"rateLimiter" jsonschema:"omitempty"`
		CircuitBreaker *resilience.CircuitBreakerRule `yaml:"circuitBreaker" jsonschema:"omitempty"`
		Retry          *resilience.RetryRule          `yaml:"retry" jsonschema:"omitempty"`
		TimeLimiter    *TimeLimiterRule               `yaml:"timeLimiter" jsonschema:"omitempty"`
		FailureCodes   []int                          `yaml:"failureCodes" jsonschema:"required,uniqueItems=true"`
	}

	// TimeLimiterRule is the spec of TimeLimiter.
	TimeLimiterRule struct {
		Timeout string `yaml:"timeout" jsonschema:"required,format=duration"`
	}

	// CanaryRule is one matching rule for canary.
	CanaryRule struct {
		ServiceInstanceLabels map[string]string               `yaml:"serviceInstanceLabels" jsonschema:"required"`
		Headers               map[string]*proxy.StringMatcher `yaml:"headers" jsonschema:"required"`
		URLs                  []*urlrule.URLRule              `yaml:"urls" jsonschema:"required"`
	}

	// ServiceCanary is the service canary entry.
	ServiceCanary struct {
		Name string `yaml:"name" jsonschema:"required"`
		// Priority must be [1, 9], the default is 5 if user does not set it.
		// The smaller number get higher priority.
		// The order is sorted by name alphabetically in the same priority.
		Priority     int              `yaml:"priority" jsonschema:"omitempty"`
		Selector     *ServiceSelector `yaml:"selector" jsonschema:"required"`
		TrafficRules *TrafficRules    `yaml:"trafficRules" jsonschema:"required"`
	}

	// TrafficRules is the rules of traffic.
	TrafficRules struct {
		Headers map[string]*proxy.StringMatcher `yaml:"headers" jsonschema:"required"`
	}

	// GlobalCanaryHeaders is the spec of global service
	GlobalCanaryHeaders struct {
		ServiceHeaders map[string][]string `yaml:"serviceHeaders" jsonschema:"omitempty"`
	}

	// GlobalTransmission is the spec of global transmission data for Agent.
	GlobalTransmission struct {
		// Headers are the canary headers, all endpoints of mesh should transmit them.
		Headers []string `yaml:"headers" jsonschema:"omitempty"`

		MomitorMTLS *MTLSTransmission `yaml:"monitorMTLS"`
	}

	// MTLSTransmission is the mTLS config for Agent.
	MTLSTransmission struct {
		CaCertBase64 string `yaml:"caCertBase64" jsonschema:"required,format=base64"`
		CertBase64   string `yaml:"certBase64" jsonschema:"required,format=base64"`
		KeyBase64    string `yaml:"keyBase64" jsonschema:"required,format=base64"`
	}

	// LoadBalance is the spec of service load balance.
	LoadBalance = proxy.LoadBalanceSpec

	// Sidecar is the spec of service sidecar.
	Sidecar struct {
		DiscoveryType   string `yaml:"discoveryType" jsonschema:"required"`
		Address         string `yaml:"address" jsonschema:"required"`
		IngressPort     int    `yaml:"ingressPort" jsonschema:"required"`
		IngressProtocol string `yaml:"ingressProtocol" jsonschema:"required"`
		EgressPort      int    `yaml:"egressPort" jsonschema:"required"`
		EgressProtocol  string `yaml:"egressProtocol" jsonschema:"required"`
	}

	// Observability is the spec of service observability.
	Observability struct {
		OutputServer *ObservabilityOutputServer `yaml:"outputServer" jsonschema:"omitempty"`
		Tracings     *ObservabilityTracings     `yaml:"tracings" jsonschema:"omitempty"`
		Metrics      *ObservabilityMetrics      `yaml:"metrics" jsonschema:"omitempty"`
	}

	// ObservabilityOutputServer is the output server of observability.
	ObservabilityOutputServer struct {
		Enabled         bool   `yaml:"enabled" jsonschema:"required"`
		BootstrapServer string `yaml:"bootstrapServer" jsonschema:"required"`
		Timeout         int    `yaml:"timeout" jsonschema:"required"`
	}

	// ObservabilityTracings is the tracings of observability.
	ObservabilityTracings struct {
		Enabled     bool                              `yaml:"enabled" jsonschema:"required"`
		SampleByQPS int                               `yaml:"sampleByQPS" jsonschema:"required"`
		Output      ObservabilityTracingsOutputConfig `yaml:"output" jsonschema:"required"`

		Request      ObservabilityTracingsDetail `yaml:"request" jsonschema:"required"`
		RemoteInvoke ObservabilityTracingsDetail `yaml:"remoteInvoke" jsonschema:"required"`
		Kafka        ObservabilityTracingsDetail `yaml:"kafka" jsonschema:"required"`
		Jdbc         ObservabilityTracingsDetail `yaml:"jdbc" jsonschema:"required"`
		Redis        ObservabilityTracingsDetail `yaml:"redis" jsonschema:"required"`
		Rabbit       ObservabilityTracingsDetail `yaml:"rabbit" jsonschema:"required"`
	}

	// ObservabilityTracingsOutputConfig is the tracing output configuration
	ObservabilityTracingsOutputConfig struct {
		Enabled         bool   `yaml:"enabled" jsonschema:"required"`
		ReportThread    int    `yaml:"reportThread" jsonschema:"required"`
		Topic           string `yaml:"topic" jsonschema:"required"`
		MessageMaxBytes int    `yaml:"messageMaxBytes" jsonschema:"required"`
		QueuedMaxSpans  int    `yaml:"queuedMaxSpans" jsonschema:"required"`
		QueuedMaxSize   int    `yaml:"queuedMaxSize" jsonschema:"required"`
		MessageTimeout  int    `yaml:"messageTimeout" jsonschema:"required"`
	}
	// ObservabilityTracingsDetail is the tracing detail of observability.
	ObservabilityTracingsDetail struct {
		Enabled       bool   `yaml:"enabled" jsonschema:"required"`
		ServicePrefix string `yaml:"servicePrefix" jsonschema:"required"`
	}

	// ObservabilityMetrics is the metrics of observability.
	ObservabilityMetrics struct {
		Enabled        bool                       `yaml:"enabled" jsonschema:"required"`
		Access         ObservabilityMetricsDetail `yaml:"access" jsonschema:"required"`
		Request        ObservabilityMetricsDetail `yaml:"request" jsonschema:"required"`
		JdbcStatement  ObservabilityMetricsDetail `yaml:"jdbcStatement" jsonschema:"required"`
		JdbcConnection ObservabilityMetricsDetail `yaml:"jdbcConnection" jsonschema:"required"`
		Rabbit         ObservabilityMetricsDetail `yaml:"rabbit" jsonschema:"required"`
		Kafka          ObservabilityMetricsDetail `yaml:"kafka" jsonschema:"required"`
		Redis          ObservabilityMetricsDetail `yaml:"redis" jsonschema:"required"`
		JvmGC          ObservabilityMetricsDetail `yaml:"jvmGc" jsonschema:"required"`
		JvmMemory      ObservabilityMetricsDetail `yaml:"jvmMemory" jsonschema:"required"`
		Md5Dictionary  ObservabilityMetricsDetail `yaml:"md5Dictionary" jsonschema:"required"`
	}

	// ObservabilityMetricsDetail is the metrics detail of observability.
	ObservabilityMetricsDetail struct {
		Enabled  bool   `yaml:"enabled" jsonschema:"required"`
		Interval int    `yaml:"interval" jsonschema:"required"`
		Topic    string `yaml:"topic" jsonschema:"required"`
	}

	// Tenant contains the information of tenant.
	Tenant struct {
		Name string `yaml:"name"`

		Services []string `yaml:"services" jsonschema:"omitempty"`
		// Format: RFC3339
		CreatedAt   string `yaml:"createdAt" jsonschema:"omitempty"`
		Description string `yaml:"description"`
	}

	// Certificate is one cert for mesh service instance or root CA.
	Certificate struct {
		IP          string `yaml:"ip" jsonschema:"required"`
		ServiceName string `yaml:"servieName" jsonschema:"required"`
		CertBase64  string `yaml:"certBase64" jsonschema:"required"`
		KeyBase64   string `yaml:"keyBase64" jsonschema:"required"`
		TTL         string `yaml:"ttl" jsonschema:"required,format=duration"`
		SignTime    string `yaml:"signTime" jsonschema:"required,format=timerfc3339"`
		HOST        string `yaml:"host" jsonschema:"required"`
	}

	// ServiceInstanceSpec is the spec of service instance.
	// FIXME: Use the unified struct: serviceregistry.ServiceInstanceSpec.
	ServiceInstanceSpec struct {
		RegistryName string `yaml:"registryName" jsonschema:"required"`
		// Provide by registry client
		ServiceName  string            `yaml:"serviceName" jsonschema:"required"`
		InstanceID   string            `yaml:"instanceID" jsonschema:"required"`
		IP           string            `yaml:"ip" jsonschema:"required"`
		Port         uint32            `yaml:"port" jsonschema:"required"`
		RegistryTime string            `yaml:"registryTime" jsonschema:"omitempty"`
		Labels       map[string]string `yaml:"labels" jsonschema:"omitempty"`

		// Set by heartbeat timer event or API
		Status string `yaml:"status" jsonschema:"omitempty"`
	}

	// IngressPath is the path for a mesh ingress rule
	IngressPath struct {
		Path          string `yaml:"path" jsonschema:"required"`
		RewriteTarget string `yaml:"rewriteTarget" jsonschema:"omitempty"`
		Backend       string `yaml:"backend" jsonschema:"required"`
	}

	// IngressRule is the rule for mesh ingress
	IngressRule struct {
		Host  string         `yaml:"host" jsonschema:"omitempty"`
		Paths []*IngressPath `yaml:"paths" jsonschema:"required"`
	}

	// Ingress is the spec of mesh ingress
	Ingress struct {
		Name  string         `yaml:"name" jsonschema:"required"`
		Rules []*IngressRule `yaml:"rules" jsonschema:"required"`
	}

	// ServiceInstanceStatus is the status of service instance.
	ServiceInstanceStatus struct {
		ServiceName string `yaml:"serviceName" jsonschema:"required"`
		InstanceID  string `yaml:"instanceID" jsonschema:"required"`
		// RFC3339 format
		LastHeartbeatTime string `yaml:"lastHeartbeatTime" jsonschema:"required,format=timerfc3339"`
	}

	// CustomResourceKind defines the spec of a custom resource kind
	CustomResourceKind = customdata.Kind

	// CustomResource defines the spec of a custom resource
	CustomResource = customdata.Data

	// HTTPMatch defines an individual route for HTTP traffic
	HTTPMatch struct {
		// Name is the name of the match for referencing in a TrafficTarget
		Name string `json:"name,omitempty"`

		// Methods for inbound traffic as defined in RFC 7231
		// https://tools.ietf.org/html/rfc7231#section-4
		Methods []string `json:"methods,omitempty"`

		// PathRegex is a regular expression defining the route
		PathRegex string `json:"pathRegex,omitempty"`

		// Headers is a list of headers used to match HTTP traffic.
		//
		// But we are unable support headers in mesh, because all requests to mesh egress
		// contains a special header 'X-Mesh-Rpc-Service', whose value is the name of the
		// target service, and the relationship between multiple headers are 'OR'.
		//
		// Headers map[string]string `json:"headers,omitempty"`
	}

	// HTTPRouteGroup defines the spec of a HTTP route group
	HTTPRouteGroup struct {
		// Name is the name for referencing a HTTPRouteGroup
		Name string `yaml:"name" jsonschema:"required"`

		// Matches is a list of HTTPMatch to match traffic
		Matches []HTTPMatch `json:"matches,omitempty"`
	}

	// TrafficTargetRule is the TrafficSpec to allow for a TrafficTarget
	TrafficTargetRule struct {
		// Kind is the kind of TrafficSpec to allow
		Kind string `json:"kind"`

		// Name of the TrafficSpec to use
		Name string `json:"name"`

		// Matches is a list of TrafficSpec routes to allow traffic for
		// +optional
		Matches []string `json:"matches,omitempty"`
	}

	// IdentityBindingSubject is a service which should be allowed access to the TrafficTarget
	IdentityBindingSubject struct {
		// Kind is the type of Subject to allow ingress (Service)
		Kind string `json:"kind"`

		// Name of the Subject, i.e. ServiceName
		Name string `json:"name"`
	}

	// TrafficTarget is the specification of a TrafficTarget
	TrafficTarget struct {
		// Name is the name for referencing a TrafficTarget
		Name string `yaml:"name" jsonschema:"required"`

		// Destination is the service to allow ingress traffic
		Destination IdentityBindingSubject `json:"destination"`

		// Sources are the services to allow egress traffic
		Sources []IdentityBindingSubject `json:"sources,omitempty"`

		// Rules are the traffic rules to allow (HTTPRoutes)
		Rules []TrafficTargetRule `json:"rules,omitempty"`
	}
)

// Validate validates ServiceCanary.
func (sc ServiceCanary) Validate() error {
	if sc.Priority < 0 || sc.Priority > 9 {
		return fmt.Errorf("invalid priority (range is [0, 9], the default 0 will be set to 5)")
	}

	return nil
}

// Clone clones TrafficRules.
func (tr *TrafficRules) Clone() *TrafficRules {
	headers := map[string]*proxy.StringMatcher{}
	for k, v := range tr.Headers {
		stringMatch := *v
		headers[k] = &stringMatch
	}

	return &TrafficRules{
		Headers: headers,
	}
}

// Validate validates Spec.
func (a Admin) Validate() error {
	switch a.RegistryType {
	case RegistryTypeConsul, RegistryTypeEureka, RegistryTypeNacos:
	default:
		return fmt.Errorf("unsupported registry center type: %s", a.RegistryType)
	}

	if a.Security != nil {
		switch a.Security.CertProvider {
		case CertProviderSelfSign:
		default:
			return fmt.Errorf("unknown mTLS cert provider type: %s", a.Security.CertProvider)
		}

		switch a.Security.MTLSMode {
		case SecurityLevelPermissive, SecurityLevelStrict:
		default:
			return fmt.Errorf("unknown mTLS security level: %s", a.Security.MTLSMode)
		}
	}

	if a.EnablemTLS() {
		appCertTTL, err := time.ParseDuration(a.Security.AppCertTTL)
		if err != nil {
			return fmt.Errorf("parse appcertTTl: %s failed: %v", a.Security.AppCertTTL, err)
		}
		rootCertTTL, err := time.ParseDuration(a.Security.RootCertTTL)
		if err != nil {
			return fmt.Errorf("parse rootTTl: %s failed: %v", a.Security.AppCertTTL, err)
		}

		if appCertTTL >= rootCertTTL {
			err = fmt.Errorf("appCertTTL: %s is larger than rootCertTTL: %s", appCertTTL.String(), rootCertTTL.String())
			return err
		}
	}

	if a.MonitorMTLS != nil {
		serviceMap := map[string]struct{}{}
		for _, cert := range a.MonitorMTLS.Certs {
			for _, service := range cert.Services {
				_, exists := serviceMap[service]
				if exists {
					return fmt.Errorf("service %s in monitotMTLS.certs occurred multiple times", service)
				}
				serviceMap[service] = struct{}{}
			}
		}
	}

	return nil
}

// EnablemTLS indicates whether we should enable mTLS in mesh or not.
func (a Admin) EnablemTLS() bool {
	if a.Security != nil && a.Security.MTLSMode == SecurityLevelStrict {
		return true
	}
	return false
}

// Key returns the key of ServiceInstanceSpec.
func (s *ServiceInstanceSpec) Key() string {
	return fmt.Sprintf("%s/%s/%s", s.RegistryName, s.ServiceName, s.InstanceID)
}

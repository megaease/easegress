# Controllers

- [Controllers](#controllers)
  - [System Controllers](#system-controllers)
    - [ServiceRegistry](#serviceregistry)
    - [TrafficController](#trafficcontroller)
    - [RawConfigTrafficController](#rawconfigtrafficcontroller)
      - [HTTPServer](#httpserver)
      - [Pipeline](#pipeline)
    - [StatusSyncController](#statussynccontroller)
  - [Business Controllers](#business-controllers)
    - [GlobalFilter](#globalfilter)
    - [EaseMonitorMetrics](#easemonitormetrics)
    - [FaaSController](#faascontroller)
    - [IngressController](#ingresscontroller)
    - [ConsulServiceRegistry](#consulserviceregistry)
    - [EtcdServiceRegistry](#etcdserviceregistry)
    - [EurekaServiceRegistry](#eurekaserviceregistry)
    - [ZookeeperServiceRegistry](#zookeeperserviceregistry)
    - [NacosServiceRegistry](#nacosserviceregistry)
    - [AutoCertManager](#autocertmanager)
  - [Common Types](#common-types)
    - [tracing.Spec](#tracingspec)
    - [zipkin.Spec](#zipkinspec)
    - [ipfilter.Spec](#ipfilterspec)
    - [httpserver.Rule](#httpserverrule)
    - [httpserver.Path](#httpserverpath)
    - [httpserver.Header](#httpserverheader)
    - [pipeline.Spec](#pipelinespec)
    - [pipeline.FlowNode](#pipelineflownode)
    - [filters.Filter](#filtersfilter)
    - [easemonitormetrics.Kafka](#easemonitormetricskafka)
    - [nacos.ServerSpec](#nacosserverspec)
    - [autocertmanager.DomainSpec](#autocertmanagerdomainspec)
    - [resilience.Policy](#resiliencepolicy)
      - [Retry Policy](#retry-policy)
      - [CircuitBreaker Policy](#circuitbreaker-policy)

As the [architecture diagram](../imgs/architecture.png) shows, the controller is the core entity to control kinds of working. There are two kinds of controllers overall:

- System Controller: It is created one and only one instance in every Easegress node, which can't be deleted. They mainly aim to control essential system-level stuff.
- Business Controller: It could be created, updated, deleted by admin operation. They control various resources such as mesh traffic, service discovery, faas, and so on.

In another view, Easegress as a traffic orchestration system, we could classify them into traffic controller and non-traffic controller:

- Traffic Controller: It invokes TrafficController to handle its specific traffic, such as MeshController.
- Non-Traffic Controller: It doesn't handle business traffic, such as EurekaServiceRegistry, even though it has admin traffic with Eureka.

The two categories are conceptual, which means they are not strict distinctions. We just use them as terms to clarify controllers technically.

## System Controllers

For now, all system controllers can not be configured. It may gain this capability if necessary in the future.

### ServiceRegistry

We use the system controller `ServiceRegistry` as the service hub for all service registries. Current drivers are

- [ConsulServiceRegistry](#consulserviceregistry)
- [EtcdServiceRegistry](#etcdserviceregistry)
- [EurekaServiceRegistry](#eurekaserviceregistry)
- [ZookeeperServiceRegistry](#zookeeperserviceregistry)
- [NacosServiceRegistry](#nacosserviceregistry)

The drivers need to offer notifying change periodically, and operations to the external service registry.

### TrafficController

TrafficController handles the lifecycle of HTTPServer and HTTPPipeline and their relationship. It manages the resource in a namespaced way. HTTPServer accepts incoming traffic and routes it to HTTPPipelines in the same namespace. Most other controllers could handle traffic by leverage the ability of TrafficController..

### RawConfigTrafficController

RawConfigTrafficController maps all traffic static configurations to TrafficController in the namespace `default`. We could use `egctl` to manage the configuration of servers and pipelines in the default namespace.

#### HTTPServer

HTTPServer is a server that listens on one port to route all traffic to available pipelines. Its simplest config looks like:

```yaml
kind: HTTPServer
name: http-server-example
port: 80
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: http-pipeline-example
```

| Name             | Type                               | Description                                                                              | Required             |
| ---------------- | ---------------------------------- | ---------------------------------------------------------------------------------------- | -------------------- |
| http3            | bool                               | Whether to support HTTP3(QUIC)                                                           | No                   |
| port             | uint16                             | The HTTP port listening on                                                               | Yes                  |
| keepAlive        | bool                               | Whether to support keepalive                                                             | Yes (default: false) |
| keepAliveTimeout | string                             | The timeout of keepalive                                                                 | Yes (default: 60s)   |
| maxConnections   | uint32                             | The max connections with clients                                                         | Yes (default: 10240) |
| https            | bool                               | Whether to use HTTPS                                                                     | Yes (default: false) |
| cacheSize        | uint32                             | The size of cache, 0 means no cache                                                      | No                   |
| xForwardedFor    | bool                               | Whether to set X-Forwarded-For header by own ip                                          | No                   |
| tracing          | [tracing.Spec](#tracingSpec)       | Distributed tracing settings                                                             | No                   |
| certBase64      | string                             | Public key of PEM encoded data in base64 encoded format                                  | No                   |
| keyBase64        | string                             | Private key of PEM encoded data in base64 encoded format                                 | No                   |
| certs            | map[string]string                  | Public keys of PEM encoded data, the key is the logic pair name, which must match keys   | No                   |
| keys             | map[string]string                  | Private keys of PEM encoded data, the key is the logic pair name, which must match certs | No                   |
| ipFilter         | [ipfilter.Spec](#ipfilterSpec)     | IP Filter for all traffic under the server                                               | No                   |
| rules            | [httpserver.Rule](#httpserverRule) | Router rules                                                                             | No                   |
| autoCert | bool | Do HTTP certification automatically | No |  
| clientMaxBodySize | int64 | Max size of request body. the default value is 4MB. Requests with a body larger than this option are discarded.  When this option is set to `-1`, Easegress takes the request body as a stream and the body can be any size, but some features are not possible in this case, please refer [Stream](./stream.md) for more information. | No | 
| caCertBase64 | string | Define the root certificate authorities that servers use if required to verify a client certificate by the policy in TLS Client Authentication. | No |
| globalFilter | string | Name of [GlobalFilter](#globalfilter) for all backends | No | 


#### Pipeline

Pipeline is used to orchestrate filters. Its simplest config looks like:

```yaml
name: http-pipeline-example1
kind: Pipeline
flow:
- filter: proxy

filters:
- name: proxy
  kind: Proxy
  pools:
  - servers:
    - url: http://127.0.0.1:9095   
```

The `flow` defines the execution order of filters. You can use `jumpIf` to change the order of execution if one filter returns unexpected `result`. For example: 

```yaml 
name: http-pipeline-example2
kind: Pipeline
# END is built in filter name, it stops execution of pipeline and returns. 
flow:
- filter: validator
  jumpIf: 
    invalid: END
- filter: proxy 

filters:
- name: validator 
  kind: Validator
  headers: 
    X-Id:
      values: ["user1", "user2"] 
- name: proxy
  kind: Proxy 
  pools:
  - servers:
    - url: http://127.0.0.1:9095 
```
In this case, if a request’s header doesn’t have the key `X-Id` or its value is not `user1` or `user2`, then the `validator` filter returns an `invalid` result and the pipeline jumps to `END`, the `proxy` filter does not execute. 


> `jumpIf` can only jump to filters behind the current filter.


The `resilience` field defines resilience policies, if a filter implements the `filters.Resiliencer` interface (for now, only the `Proxy` filter implements the interface), the pipeline injects the policies into the filter instance after creating it.
A filter can implement the `filters.Resiliencer` interface to support resilience. There are two kinds of resilience, `Retry` and `CircuitBreaker`. Check [resilience](#resilience) for more details. The following config adds a retry policy to the proxy filter: 
```yaml
name: http-pipeline-example3
kind: Pipeline
flow:
- filter: proxy 

filters:
- name: proxy
  kind: Proxy 
  pools:
  - servers:
    - url: http://127.0.0.1:9095
    retryPolicy: retry
    
resilience:
- name: retry
  kind: Retry
  maxAttempts: 3
``` 
In this case, if `proxy` returns non-empty results, then resilience retry reruns the `proxy` filter until `proxy` returns empty results or gets the max attempts. 

The `flow` also supports `namespace`, so the pipeline can support workflows that contain multiple requests and responses. 
```yaml
name: http-pipeline-example4
kind: Pipeline
flow: 
- filter: validator
  jumpIf:
    invalid: END 
- filter: requestBuilderFoo 
  namespace: foo 
- filter: proxyFoo 
  namespace: foo 
- filter: requestBuilderBar 
  namespace: bar 
- filter: proxyBar
  namespace: bar 
- filter: responseBuilder 

filters: 
- name: requestBuilder
  kind: RequestBuilder
  ...
... 
```
In this case, `requestBuilderFoo` creates a request in namespace `foo`, and `proxyFoo` sends `foo` request and puts the response into namespace `foo`. `requestBuilderBar` creates a request in namespace `bar` and `proxyBar` sends `bar` request and puts the response into namespace `bar`. Finally, `requestBuilder` creates a response and puts it into the default namespace. 

> If not set, the filter works in the default namespace `DEFAULT`.


The `alias` in `flow` gives a filter an alias to help re-use the filter so that we can use the alias to distinguish each of its appearances in the flow.

```yaml
name: http-pipeline-example5
kind: Pipeline
flow:
- filter: validator
  jumpIf:
    invalid: proxy2
- filter: proxy
# when meeting filter END, the pipeline execution stops and returns.
- filter: END
- filter: proxy
  alias: proxy2
- filter: responseAdaptor

filters:
- name: proxy
  kind: Proxy
  ...
```
In this case, we give second `proxy` alias `proxy2`, so request is invalid, it jumps to second proxy. 

The `data` field defines static user data for the pipeline, which can be
accessed by filters. For example, in the below pipeline, the body of the result
request of the RequestBuilder will be `hello world`, which is the value of
data item `foo`.

```yaml
name: http-pipeline-example6
kind: Pipeline 
flow:
  ...

filters:
- name: requestBuilder
  kind: RequestBuilder
  template: |
    body: {{.data.PIPELINE.foo}}

data:
  foo: "hello world"
```

| Name          | Type     | Description    | Required             |
| ------------- | -------- | -------------- | -------------------- |
| flow       | [][FlowNode](#pipelineflownode)  | The execution order of filters, if empty, will use the order of the filter definitions. | No  |
| filters    | []map[string]interface{}         | Defines filters, please refer [Filters](filters.md) for details of a specific filter kind.     | Yes |
| resilience | []map[string]interface{}         | Defines resilience policies, please refer [Resilience Policy](#resiliencepolicy) for details of a specific resilience policy.    | No |
| data       | map[string]interface{}           | Static user data of the pipeline.         | No  |

### StatusSyncController

No config.

## Business Controllers

### GlobalFilter

`GlobalFilter` is a special pipeline that can be executed before or/and after all pipelines in a server. For example:  

```yaml
name: globalFilter-example
kind: GlobalFilter
beforePipeline:
  flow: 
  - filter: validator

  filters: 
  - name: validator
    kind: Validator 
    ...
---
name: server-example 
kind: HTTPServer
globalFilter: globalFilter-example
...
```
In this case, all requests in HTTPServer `server-example` go through GlobalFilter `globalFilter-example` before executing any other pipelines. 


| Name | Type | Description | Required | 
|------|------|-------------|----------|
| beforePipeline | [pipeline.Spec](#pipelineSpec) | Spec for before pipeline | No |
| afterPipeline | [pipeline.Spec](#pipelinespec) | Spec for after pipeline | No | 


### EaseMonitorMetrics

EaseMonitorMetrics is adapted to monitor metrics of Easegress and send them to Kafka. The config looks like:

```yaml
kind: EaseMonitorMetrics
name: easemonitor-metrics-example
kafka:
  brokers: ["127.0.0.1:9092"]
  topic: metrics
```

| Name  | Type                                                 | Description          | Required |
| ----- | ---------------------------------------------------- | -------------------- | -------- |
| kafka | [easemonitormetrics.Kafka](#easemonitormetricsKafka) | Kafka related config | Yes      |

### FaaSController

A FaaSController is a business controller for handling Easegress and FaaS products integration purposes.  It abstracts `FaasFunction`, `FaaSStore` and, `FaasProvider`. Currently, we only support `Knative` type `FaaSProvider`.

For the full reference document please check - [FaaS Controller](./faascontroller.md)

### IngressController

The IngressController is an implementation of [Kubernetes ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/), it watches Kubernetes Ingress, Service, Endpoints, and Secrets then translates them to Easegress HTTP server and pipelines. The config looks like:

```yaml
kind: IngressController
name: ingress-controller-example
kubeConfig:
masterURL:
namespaces: ["default"]
ingressClass: easegress
httpServer:
  port: 8080
  https: false
  keepAlive: true
  keepAliveTimeout: 60s
  maxConnections: 10240
```

| Name         | Type                           | Description                                                                                                                                                               | Required                |
| ------------ | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- |
| kubeConfig   | string                         | Path of the Kubernetes configuration file.                                                                                                                                | No                      |
| masterURL    | string                         | The address of the Kubernetes API server.                                                                                                                                 | No                      |
| namespaces   | []string                       | An array of Kubernetes namespaces which the IngressController needs to watch, all namespaces are watched if left empty.                                                   | No                      |
| ingressClass | string                         | The IngressController only handles `Ingresses` with `ingressClassName` set to the value of this option.                                                                   | No (default: easegress) |
| httpServer   | [httpserver.Spec](#httpserver) | Basic configuration for the shared HTTP traffic gate. The routing rules will be generated dynamically according to Kubernetes ingresses and should not be specified here. | Yes                     |

**Note**: IngressController uses `kubeConfig` and `masterURL` to connect to Kubernetes, at least one of them must be specified when deployed outside of a Kubernetes cluster, and both are optional when deployed inside a cluster.


### ConsulServiceRegistry

ConsulServiceRegistry supports service discovery for Consul as backend. The config looks like:

```yaml
kind: ConsulServiceRegistry
name: consul-service-registry-example
address: '127.0.0.1:8500'
scheme: http
syncInterval: 10s
```

| Name         | Type     | Description                  | Required                      |
| ------------ | -------- | ---------------------------- | ----------------------------- |
| address      | string   | Consul server address        | Yes (default: 127.0.0.1:8500) |
| scheme       | string   | Communication scheme         | Yes (default: http)           |
| datacenter   | string   | Datacenter name              | No                            |
| token        | string   | ACL token for communication  | No                            |
| Namespace    | string   | Namespace to use             | No                            |
| syncInterval | string   | Interval to synchronize data | Yes (default: 10s)            |
| serviceTags  | []string | Service tags to query        | No                            |

### EtcdServiceRegistry

EtcdServiceRegistry support service discovery for Etcd as backend. The config looks like:

```yaml
kind: EtcdServiceRegistry
name: etcd-service-registry-example
endpoints: ['127.0.0.1:12379']
prefix: "/services/"
cacheTimeout: 10s
```

| Name         | Type     | Description                    | Required                  |
| ------------ | -------- | ------------------------------ | ------------------------- |
| endpoints    | []string | Endpoints of Etcd servers      | Yes                       |
| prefix       | string   | Prefix of the keys of services | Yes (default: /services/) |
| cacheTimeout | string   | Timeout of cache               | Yes (default: 60s)        |

### EurekaServiceRegistry

EurekaServiceRegistry supports service discovery for Eureka as backend. The config looks like:

```yaml
kind: EurekaServiceRegistry
name: eureka-service-registry-example
endpoints: ['http://127.0.0.1:8761/eureka']
syncInterval: 10s
```

| Name         | Type     | Description                  | Required                                    |
| ------------ | -------- | ---------------------------- | ------------------------------------------- |
| endpoints    | []string | Endpoints of Eureka servers  | Yes (default: http://127.0.0.1:8761/eureka) |
| syncInterval | string   | Interval to synchronize data | Yes (default: 10s)                          |

### ZookeeperServiceRegistry

ZookeeperServiceRegistry supports service discovery for Zookeeper as backend. The config looks like:

```yaml
kind: ZookeeperServiceRegistry
name: zookeeper-service-registry-example
zkservices: [127.0.0.1:2181]
prefix: /services
conntimeout: 6s
syncInterval: 10s
```

| Name         | Type     | Description                  | Required                      |
| ------------ | -------- | ---------------------------- | ----------------------------- |
| zkservices   | []string | Zookeeper service addresses  | Yes (default: 127.0.0.1:2181) |
| connTimeout  | string   | Timeout of connection        | Yes (default: 6s)             |
| Prefix       | string   | Prefix of services           | Yes (default: /)              |
| syncInterval | string   | Interval to synchronize data | Yes (default: 10s)            |

### NacosServiceRegistry

NacosServiceRegistry supports service discovery for Nacos as backend. The config looks like:

```yaml
kind: NacosServiceRegistry
name: nacos-service-registry-example
syncInterval: 10s
servers:
  - scheme: http
    port: 8848
    contextPath: /nacos
    ipAddr: 127.0.0.1
```

| Name         | Type                                  | Description                  | Required           |
| ------------ | ------------------------------------- | ---------------------------- | ------------------ |
| servers      | [][nacosServerSpec](#nacosserverspec) | Servers of Nacos             | Yes                |
| syncInterval | string                                | Interval to synchronize data | Yes (default: 10s) |
| namespace    | string                                | The namespace of Nacos       | No                 |
| username     | string                                | The username of client       | No                 |
| password     | string                                | The password of client       | No                 |

### AutoCertManager

AutoCertManager automatically manage HTTPS certificates. The config looks like:

```yaml
kind: AutoCertManager
name: autocert
email: someone@megaease.com
directoryURL: https://acme-v02.api.letsencrypt.org/directory
renewBefore: 720h
enableHTTP01: true
enableTLSALPN01: true
enableDNS01: true
domains:
  - name: "*.megaease.com"
    dnsProvider:
      name: alidns
      zone: megaease.com
```

| Name            | Type                                       | Description                                                                          | Required                           |
| --------------- | ------------------------------------------ | ------------------------------------------------------------------------------------ | ---------------------------------- |
| email           | string                                     | An email address for CA account                                                      | Yes                                |
| directoryURL    | string                                     | The endpoint of the CA directory                                                     | No (default to use Let's Encrypt)  |
| renewBefore     | string                                     | A certificate will be renewed before this duration of its expire time                | No (default 720 hours)             |
| enableHTTP01    | bool                                       | Enable HTTP-01 challenge (Easegress need to be accessable at port 80 when true)      | No (default true)                  |
| enableTLSALPN01 | bool                                       | Enable TLS-ALPN-01 challenge (Easegress need to be accessable at port 443 when true) | No (default true)                  |
| enableDNS01     | bool                                       | Enable DNS-01 challenge                                                              | No (default true)                  |
| domains         | [][DomainSpec](#autocertmanagerdomainspec) | Domains to be managed                                                                | Yes                                |

## Common Types

### tracing.Spec

| Name        | Type                       | Description                   | Required |
| ----------- | -------------------------- | ----------------------------- | -------- |
| serviceName | string                     | The service name of top level | Yes      |
| tags        | map[string]string          | Tags to include to every span | No       |
| Zipkin      | [zipkin.Spec](#zipkinSpec) | The tracing spec of zipkin    | No       |

### zipkin.Spec

| Name       | Type    | Description                                                                                        | Required |
| ---------- | ------- | -------------------------------------------------------------------------------------------------- | -------- |
| hostPort   | string  | The host:port of the service                                                                       | No       |
| serverURL  | string  | The zipkin server URL                                                                              | Yes      |
| sampleRate | float64 | The sample rate for collecting metrics, the range is [0, 1]                                        | Yes      |
| sameSpan   | bool    | Whether to allow to place client-side and server-side annotations for an RPC call in the same span | No       |
| id128Bit   | bool    | Whether to start traces with 128-bit trace id                                                      | No       |

### ipfilter.Spec

| Name           | Type     | Description                                          | Required             |
| -------------- | -------- | ---------------------------------------------------- | -------------------- |
| blockByDefault | bool     | Set block is the default action if not matching      | Yes (default: false) |
| allowIPs       | []string | IPs to be allowed to pass (support IPv4, IPv6, CIDR) | No                   |
| blockIPs       | []string | IPs to be blocked to pass (support IPv4, IPv6, CIDR) | No                   |

### httpserver.Rule

| Name       | Type                               | Description                                                   | Required |
| ---------- | ---------------------------------- | ------------------------------------------------------------- | -------- |
| ipFilter   | [ipfilter.Spec](#ipfilterSpec)     | IP Filter for all traffic under the rule                      | No       |
| host       | string                             | Exact host to match, empty means to match all                 | No       |
| hostRegexp | string                             | Host in regular expression to match, empty means to match all | No       |
| paths      | [httpserver.Path](#httpserverPath) | Path matching rules, empty means to match nothing             | No       |

### httpserver.Path

| Name          | Type                                     | Description                                                                                                                            | Required |
| ------------- | ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| ipFilter      | [ipfilter.Spec](#ipfilterSpec)           | IP Filter for all traffic under the path                                                                                               | No       |
| path          | string                                   | Exact path to match                                                                                                                    | No       |
| pathPrefix    | string                                   | Prefix of the path to match                                                                                                            | No       |
| pathRegexp    | string                                   | Path in regular expression to match                                                                                                    | No       |
| rewriteTarget | string                                   | Use pathRegexp.[ReplaceAllString](https://golang.org/pkg/regexp/#Regexp.ReplaceAllString)(path, rewriteTarget) or pathPrefix [strings.Replace](https://pkg.go.dev/strings#Replace) to rewrite request path | No       |
| methods       | []string                                 | Methods to match, empty means to allow all methods                                                                                     | No       |
| headers       | [][httpserver.Header](#httpserverHeader) | Headers to match (the requests matching headers won't be put into cache)                                                               | No       |
| backend       | string                                   | backend name (pipeline name in static config, service name in mesh)                                                                    | Yes      |
| clientMaxBodySize | int64 | Max size of request body, will use the option of the HTTP server if not set. the default value is 4MB. Requests with a body larger than this option are discarded.  When this option is set to `-1`, Easegress takes the request body as a stream and the body can be any size, but some features are not possible in this case, please refer [Stream](./stream.md) for more information. | No | 
| matchAllHeader | bool | Match all headers that are defined in headers, default is `false`. | No |


### httpserver.Header

There must be at least one of `values` and `regexp`.

| Name    | Type     | Description                                                         | Required |
| ------- | -------- | ------------------------------------------------------------------- | -------- |
| key     | string   | Header key to match                                                 | Yes      |
| values  | []string | Header values to match                                              | No       |
| regexp  | string   | Header value in regular expression to match                         | No       |

### pipeline.Spec 
| Name | Type | Description | Required | 
|------|------|-------------|----------|
| flow | [pipeline.FlowNode](#pipelineFlowNode) | Flow of pipeline | No |
| filters | [][filters.Filter](#filters.Filter) | Filter definitions of pipeline  | Yes |
| resilience | [][resilience.Policy](#resiliencePolicy) | Resilience policy for backend filters | No | 

### pipeline.FlowNode

| Name   | Type              | Description                                                                                                                                                                         | Required |
| ------ | ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| filter | string            | The filter name                                                                                                                                                                     | Yes      |
| jumpIf | map[string]string | Jump to another filter conditionally, the key is the result of the current filter, the value is the jumping filter name. `END` is the built-in value for the ending of the pipeline | No       |
| namespace | string | Namespace of the filter | No | 
| alias | string | Alias name of the filter | No | 

### filters.Filter

The self-defining specification of each filter references to [filters](./filters.md).

| Name                                 | Type   | Description    | Required |
| ------------------------------------ | ------ | -------------- | -------- |
| name                                 | string | Name of filter | Yes      |
| kind                                 | string | Kind of filter | Yes      |
| [self-defining fields](./filters.md) | -      | -              | -        |

### easemonitormetrics.Kafka

| Name    | Type     | Description      | Required                      |
| ------- | -------- | ---------------- | ----------------------------- |
| brokers | []string | Broker addresses | Yes (default: localhost:9092) |
| topic   | string   | Produce topic    | Yes                           |

### nacos.ServerSpec

| Name        | Type   | Description                                  | Required |
| ----------- | ------ | -------------------------------------------- | -------- |
| ipAddr      | string | The ip address                               | Yes      |
| port        | uint16 | The port                                     | Yes      |
| scheme      | string | The scheme of protocol (support http, https) | No       |
| contextPath | string | The context path                             | No       |

### autocertmanager.DomainSpec

| Name        | Type              | Description               | Required                             |
| ----------- | ----------------- | --------------------------| ------------------------------------ |
| name        | string            | The name of the domain    | Yes                                  |
| dnsProvider | map[string]string | DNS provider information  | No (Yes if `name` is a wildcard one) |

The fields in `dnsProvider` vary from DNS providers, but `name` and `zone` are required for all DNS providers.
Below table list other required fields for each supported DNS provider (Note: `google` is temporarily disabled due to dependency conflict):

| DNS Provider Name | Required Fields                                                     |
| ----------------- | ------------------------------------------------------------------- |
| alidns            | accessKeyId, accessKeySecret                                        |
| azure             | tenantId, clientId, clientSecret, subscriptionId, resourceGroupName |
| cloudflare        | apiToken                                                            |
| digitalocean      | apiToken                                                            |
| dnspod            | apiToken                                                            |
| duckdns           | apiToken                                                            |
| google            | project                                                             |
| hetzner           | authApiToken                                                        |
| route53           | accessKeyId, secretAccessKey, awsProfile                            |
| vultr             | apiToken                                                            |


### resilience.Policy

| Name                 | Type   | Description    | Required |
| -------------------- | ------ | -------------- | -------- |
| name                 | string | Name of filter | Yes      |
| kind                 | string | Kind of filter | Yes      |
| other kind specific fields of the policy kind | -      | -              | -        |

#### Retry Policy
A retry policy configures how to retry a failed request.

| Name | Type | Description | Required |
|------|------|-------------|----------|
| maxAttempts | int | The maximum number of attempts (including the initial one). Default is 3 | No |
| waitDuration | string | The base wait duration between attempts. Default is 500ms | No |
| backOffPolicy | string  | The back-off policy for wait duration, could be `EXPONENTIAL` or `RANDOM` and the default is `RANDOM`. If configured as `EXPONENTIAL`, the base wait duration becomes 1.5 times larger after each failed attempt | No |
| randomizationFactor  | float64 | Randomization factor for actual wait duration, a number in interval `[0, 1]`, default is 0. The actual wait duration used is a random number in interval `[(base wait duration) * (1 - randomizationFactor),  (base wait duration) * (1 + randomizationFactor)]` | No |

#### CircuitBreaker Policy

CircuitBreaker leverges a finite state machine to implement the processing logic, the state machine has three states: `CLOSED`, `OPEN`, and `HALF_OPEN`. When the state is `CLOSED`, requests pass through normally, state transits to `OPEN` if request failure rate or slow request rate reach a configured threshold and requests will be shor-circuited in this state. After a configured duration, state transits from `OPEN` to `HALF_OPEN`, in which a limited number of requests are permitted to pass through while other requests are still short-circuited, and state transit to `CLOSED` or `OPEN`
based on the results of the permitted requests.

When `CLOSED`, it uses a sliding window to store and aggregate the result of recent requests, the window can either be `COUNT_BASED` or `TIME_BASED`. The `COUNT_BASED` window aggregates the last N requests and the `TIME_BASED` window aggregates requests in the last N seconds, where N is the window size.

Below is an example configuration with both `COUNT_BASED` and `TIME_BASED` policies. 

Policy `circuit-breaker-example-count` short-circuits requests if more than half of recent requests failed.

Policy `circuit-breaker-example-time` short-circuits requests if more than 60% of recent requests failed.

> failed means that backend filter returns non-empty results. 

```yaml
kind: CircuitBreaker
name: circuit-breaker-example-count 
slidingWindowType: COUNT_BASED
failureRateThreshold: 50
slidingWindowSize: 100

---
kind: CircuitBreaker
name: circuit-breaker-example-time 
slidingWindowType: TIME_BASED
failureRateThreshold: 60
slidingWindowSize: 100
```

| Name | Type | Description | Required | 
|------|------|-------------|----------| 
| slidingWindowType | string | Type of the sliding window which is used to record the outcome of requests when the CircuitBreaker is `CLOSED`. Sliding window can either be `COUNT_BASED` or `TIME_BASED`. If the sliding window is `COUNT_BASED`, the last `slidingWindowSize` requests are recorded and aggregated. If the sliding window is `TIME_BASED`, the requests of the last `slidingWindowSize` seconds are recorded and aggregated. Default is `COUNT_BASED` | No |
| failureRateThreshold | int8 | Failure rate threshold in percentage. When the failure rate is equal to or greater than the threshold the CircuitBreaker transitions to `OPEN` and starts short-circuiting requests. Default is 50 | No | 
| slowCallRateThreshold | int8 | Slow rate threshold in percentage. The CircuitBreaker considers a request as slow when its duration is greater than `slowCallDurationThreshold`. When the percentage of slow requests is equal to or greater than the threshold, the CircuitBreaker transitions to `OPEN` and starts short-circuiting requests. Default is 100 | No |
| slowCallDurationThreshold | string | Duration threshold for slow call | No | 
| slidingWindowSize | uint32 | The size of the sliding window which is used to record the outcome of requests when the CircuitBreaker is `CLOSED`. Default is 100 | No |
| permittedNumberOfCallsInHalfOpenState | uint32 | The number of permitted requests when the CircuitBreaker is `HALF_OPEN`. Default is 10 | No |
| minimumNumberOfCalls | uint32 | The minimum number of requests which are required (per sliding window period) before the CircuitBreaker can calculate the error rate or slow requests rate. For example, if `minimumNumberOfCalls` is 10, then at least 10 requests must be recorded before the failure rate can be calculated. If only 9 requests have been recorded the CircuitBreaker will not transition to `OPEN` even if all 9 requests have failed. Default is 10 | No |
| maxWaitDurationInHalfOpenState | string | The maximum wait duration which controls the longest amount of time a CircuitBreaker could stay in `HALF_OPEN` state before it switches to `OPEN`. Value 0 means CircuitBreaker would wait infinitely in `HALF_OPEN` State until all permitted requests have been completed. Default is 0| No |
| waitDurationInOpenState | string | The time that the CircuitBreaker should wait before transitioning from `OPEN` to `HALF_OPEN`. Default is 60s | No |

See more details about `Retry`, `CircuitBreaker` or other resilience polcies in [here](../cookbook/resilience.md).

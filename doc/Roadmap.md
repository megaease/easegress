# Easegress Roadmap

- [Easegress Roadmap](#easegress-roadmap)
  - [Product Principles](#product-principles)
  - [Features](#features)
    - [Business Extensibility](#business-extensibility)
    - [Traffic Orchestration](#traffic-orchestration)
  - [Roadmap 2022](#roadmap-2022)
    - [Business Extensibility](#business-extensibility-1)
    - [Traffic Orchestration](#traffic-orchestration-1)

## Product Principles
1. **Traffic Orchestration**. It must be **traffic-specific**. It tries to solve users’ traffic-based requirements and solutions. Easegress can empower customer business capabilities, e.g., using Easegress to support high concurrent traffic scenarios(such as Flash-Sale, Black Friday, Double 11 event, etc).  And enhancing the APIs orchestration & management.
  
2. **Opening & Extensibility**. It aims to be an **extensible-development** platform. Users can organize the existing filters into a pipeline, or completely customize a brand-new filter/controller for their specific business logic. With simple, clean, and flat software architecture, anyone can develop their own filter/controller/pipeline rapidly and easily. 
  
3. **Cloud Native**. It is designed to be **cloud-native** compliance. It's scalable, resilient, manageable, and observable. It's easy to integrate with cloud-native architecture, such as Spring Cloud, Service Discovery, Service Gateway, Tracing, Kubernetes, Serverless/FaaS, and so on.

## Features
Based on our product principles, we have made a classification of Easegress' features for powering users' business capabilities into two categories: business-specific and traffic-specific.

### Business Extensibility
* Dynamically load business code written in any language with the lowest performance costing.
* Easy to develop new features with Battery-included Easegress.
* Reusability, such as using Easegress as K8s ingress, Service Gateway, Traffic gateway.
* Easy to operate/easy to install.

### Traffic Orchestration 
* Supporting traffic management - load balance, rate limiting, etc. 
* Supporting super-high concurrent requests scenario,e.g., Flash-sale, Black Friday, Double 11 event.
* Friendly API orchestration - API aggression, API pipeline 
* Protecting core APIs from high traffic load.
* Supporting canary development and traffic coloring.
* Filtering out invalid traffic for the backend.

## Roadmap 2022
### Business Extensibility

| Name                           | Issue                                                    | Description                                                                                                    |
| ------------------------------ | -------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| More enterprise user cases     |                                                          | API and traffic orchestration for complex tasks and enterprise user cases.                                                            |
| Configurable System Controller | [#397](https://github.com/megaease/easegress/issues/397) | Make the system controllers support dynamic configuration.                                                     |
| Enhanced Validation Message    |                                                          | Make JSON schema validation messages more readable, e.g., include item index.                                  |
| Operation Enhancement          |                                                          | For better supporting daily cluster operation routine,e.g., one-click installation supported, ansible script, backup and restore. |
| Observability                  |                                                          | Support Zipkin tracing (B3 protocol), upload metrics to Prometheus.                                            |
| Controller/Filter Versioning   |                                                          | Configuring  `Controller/Filter` with specified versions.                                                      |
| Protobuf Models Generating     |                                                          | Generating Easegress inner models and related docs with pre-defined Protobuf.                                  |
| Unit Testing                   |                                                          | Eliminate global components for better unit testing experience, improve unit test coverage.                    |
| Modularized Logger             |                                                          | Make logger modularized and automatical environment perception.                                                |
| Admin API to support JSON      |                                                          | Admin API to support JSON input and output, in addition to YAML.                                             |

###  Traffic Orchestration 

| Name                           | Issue                                                    | Description                                                                                                    |
| ------------------------------ | -------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| HTTP basic authentication      | [#404](https://github.com/megaease/easegress/issues/404) | Using HTTP basic authentication to restrict access.                                                            |
| Universal Pipeline             |                                                          | Pipeline to support multiple protocol.                                                                         |
| Traffic coloring               |                                                          | Supporting coloring ingress traffic by adding a special HTTP header according to users' model.                 |
| Layer 4 Proxy                  |                                                          | TCP & UDP                                                                                                      |
| More protocol supporting       |                                                          | Such as gRPC..                                                                                                 |
| mTLS for more protocol         |                                                          | Such as MQTT                                                                                                      |
| Benchmark                      |                                                          | Reach 85% of the nginx QPS when using a mocked backend.                                                        |
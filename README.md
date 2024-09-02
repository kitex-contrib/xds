# xDS support
English | [中文](README_CN.md)

[xDS](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol) is a set of discovery services that enables date-plane to discover various dynamic resources via querying from the management servers.

This project adds xDS support for Kitex and enables Kitex to perform in Proxyless mode. For more details of the design, please refer to the [proposal](https://github.com/cloudwego/kitex/issues/461).

## Feature

* Service Discovery
* Traffic Route: supports `exact` `prefix` `regex` match for `header` and `method`
    * [HTTP route configuration](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/http/http_routing#arch-overview-http-routing): configure via [VirtualService](https://istio.io/latest/docs/reference/config/networking/virtual-service/).
      * Since Istio provides limited support for thrift protocol and most users are familiar with the config of VirtualService, we use a mapping from HTTP to Thrift in this configuration. 
    * [ThriftProxy](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/thrift_proxy/v3/thrift_proxy.proto): configure via patching [EnvoyFilter](https://istio.io/latest/docs/reference/config/networking/envoy-filter/).
      * The configuration in ThriftProxy is very limited.
* Timeout Configuration:
    * Configuration inside HTTP route configuration: configure via VirtualService.

* CircuitBreaking
    * Configuration inside [Cluster](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/cluster/v3/outlier_detection.proto) configuration: configure via EnvoyFilter.

* RateLimit
    * Configuration inside [Local rate Limit](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/local_rate_limit_filter) configuration: configure via EnvoyFilter.

## Usage
There are two steps for enabling xDS for Kitex applications: 1. xDS module initialization and 2. Kitex Client/Server Option configuration.

### xDS module
To enable xDS mode in Kitex, we should invoke `xds.Init()` to initialize the xds module, including the `xdsResourceManager` and `xdsClient`.

#### Bootstrap
The xdsClient is responsible for the interaction with the xDS Server (i.e. Istio). It needs some environment variables for initialization, which need to be set inside the `spec.containers.env` of the Kubernetes Manifest file in YAML format.

*  `POD_NAMESPACE`: the namespace of the current service.
*  `POD_NAME`: the name of this pod.
*  `INSTANCE_IP`: the ip of this pod.
*  `KITEX_XDS_METAS`: the metadata of this xDS node.

Add the following part to the definition of your container that uses xDS-enabled Kitex client.

```
- name: POD_NAMESPACE
valueFrom:
  fieldRef:
    fieldPath: metadata.namespace
- name: POD_NAME
valueFrom:
  fieldRef:
    fieldPath: metadata.name
- name: INSTANCE_IP
valueFrom:
  fieldRef:
    fieldPath: status.podIP
- name: KITEX_XDS_METAS
  value: '{"CLUSTER_ID":"Kubernetes","DNS_AUTO_ALLOCATE":"true","DNS_CAPTURE":"true","INSTANCE_IPS":"$(INSTANCE_IP)","NAMESPACE":"$(POD_NAMESPACE)"}'
```

### Client-side

For now, we only provide the support on the client-side. 
To use a xds-enabled Kitex client, you should specify `destService` using the URL of your target service and add one option `WithXDSSuite`.

* Construct a `xds.ClientSuite` that includes `RouteMiddleware` and `Resolver`, and then pass it into the `WithXDSSuite` option.

```
// import "github.com/cloudwego/kitex/pkg/xds"

client.WithXDSSuite(xds.ClientSuite{
	RouterMiddleware: xdssuite.NewXDSRouterMiddleware(),
	Resolver:         xdssuite.NewXDSResolver(),
}),
```

* The URL of the target service should be in the format, which follows the format in [Kubernetes](https://kubernetes.io/):

```
<service-name>.<namespace>.svc.cluster.local:<service-port>
<service-name>.<namespace>.svc:<service-port>
<service-name>.<namespace>:<service-port>
<service-name>:<service-port> // access the <service-name> in same namespace.
```

#### Traffic route based on Tag Match

We can define traffic route configuration via [VirtualService](https://istio.io/latest/docs/reference/config/networking/virtual-service/) in Istio.

The following example indicates that when the tag contains {"stage":"canary"} in the header, the request will be routed to the `v1` subcluster of `kitex-server`.

```
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: kitex-server
spec:
  hosts:
    - kitex-server
  http:
  - name: "route-based-on-tags"
    match:
      - headers:
          stage:
            exact: "canary"
      # support prefix match
      - headers:
          stage:
            prefix: "can"
      # support regex match
      - headers:
          stage:
            regex: "[canary|prod]"
    route:
    - destination:
        host: kitex-server
        subset: v1
      weight: 100
    timeout: 0.5s
```
The example of prefix and regex match like this:
```
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: kitex-server
spec:
  hosts:
    - kitex-server
  http:
  - name: "route-based-on-tags"
    match:
      # support prefix match
      - headers:
          stage:
            prefix: "can"
      # support regex match
      - headers:
          stage:
            regex: "[canary|prod]"
    route:
    - destination:
        host: kitex-server
        subset: v1
      weight: 100
    timeout: 0.5s
```

In order to match the rules defined in the VirtualService we need to specify the tags (metadata) of the traffic which will be used to match the rules.

For example: set the key and value to "stage" and "canary" to match the above rules defined in VirtualService.

* We can first define a MetaExtractor and pass it to `RouterMiddleware` through `xdssuite.WithRouterMetaExtractor`.
  * Notice: If RouterMetaExtractor is not configured, metainfo.GetAllValues will be used by default.
```
var (
	routeKey   = "stage"
	routeValue = "canary"
)

func routeByStage(ctx context.Context) map[string]string {
	if v, ok := metainfo.GetValue(ctx, routeKey); ok {
		return map[string]string{
			routeKey: v,
		}
	}
	return nil
}

// add the option
client.WithXDSSuite(xds2.ClientSuite{
	RouterMiddleware: xdssuite.NewXDSRouterMiddleware(
		// use this option to specify the meta extractor
		xdssuite.WithRouterMetaExtractor(routeByStage),
	),
	Resolver: xdssuite.NewXDSResolver(),
}),
```
* Set the metadata of the traffic (corresponding to the MetaExtractor) when make RPC Call. 
Here, we use `metainfo.WithValue` to specify the label of the traffic. Metadata will be extracted for route matching.
```
ctx := metainfo.WithValue(context.Background(), routeKey, routeValue)
```

#### Traffic route based on Method Match

Same as above, using [VirtualService](https://istio.io/latest/docs/reference/config/networking/virtual-service/) in Istio to define traffic routing configuration.

The example below shows that requests with method equal to SayHello are routed to the `v1` subcluster of `kitex-server`. It should be noted that when defining rules, you need to include package name and service name, corresponding to `namespace` and `service` in thrift idl.

* uri:  `/${PackageName}.${ServiceName}/${MethodName}`

```
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: kitex-server
spec:
  hosts:
    - kitex-server
  http:
  - name: "route-based-on-path"
    match:
      - uri:
          # /${PackageName}.${ServiceName}/${MethodName}
          exact: /proxyless.GreetService/SayHello
    route:
    - destination:
        host: kitex-server
        subset: v2
      weight: 100
    timeout: 0.5s
```

#### CircuitBreaking

```
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: circuitbreak
  # the namespace of pod
  namespace: default
spec:
  configPatches:
  - applyTo: CLUSTER
    match:
      context: SIDECAR_OUTBOUND
      cluster:
        # serviceName + namespace + .svc.cluster.local 
        service: kitex-server.default.svc.cluster.local
    patch:
      operation: MERGE
      value:
        outlier_detection:
          # the failure percentage threshold
          failure_percentage_threshold: 10
          # the failure percentage request volume
          failure_percentage_request_volume: 101
  workloadSelector:
    labels:
      # the label of the client pod.
      app.kubernetes.io/name: kitex-client
```

#### RateLimit

```
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: filter-local-ratelimit-svc
  namespace: default
spec:
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: SIDECAR_INBOUND
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
      patch:
        operation: INSERT_BEFORE
        value:
          name: envoy.filters.http.local_ratelimit
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.filters.http.local_ratelimit.v3.LocalRateLimit
            stat_prefix: http_local_rate_limiter
            token_bucket:
              # the qps limit
              max_tokens: 4
  workloadSelector:
    labels:
      # the label of the server pod.
      app.kubernetes.io/name: kitex-server

```

#### Retry

Support using VirtualService and EnvoyFilter to config retry policy, the EnvoyFilter has more configuration.

```
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: retry-sample
  namespace: default
spec:
  hosts:
  - hello.prod.svc.cluster.local:21001
  http:
  - route:
    - destination:
        host: hello.prod.svc.cluster.local:21001
    retries:
      attempts: 1
      perTryTimeout: 2s
```

```
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: retry-enhance
  namespace: default
spec:
  configPatches:
  - applyTo: HTTP_ROUTE
    match:
      context: SIDECAR_OUTBOUND
      routeConfiguration:
        # service name, should obey FQDN
        name: hello.default.svc.cluster.local:21001
        vhost: 
          # service name, should obey FQDN
          name: hello.default.svc.cluster.local:21001
    patch:
      operation: MERGE
      value:
        route:
          retryPolicy:
            numRetries: 3
            perTryTimeout: 100ms
            retryBackOff:
              baseInterval: 100ms
              maxInterval: 100ms
            retriableHeaders:
              - name: "kitexRetryErrorRate"
                stringMatch:
                  exact: "0.29"
              - name: "kitexRetryMethods"
                stringMatch:
                  exact: "Echo,Greet"
  workloadSelector:
    labels:
      # the label of the service pod.
      app.kubernetes.io/name: kitex-client
```

## Example
The usage is as follows:

```
import (
	"github.com/cloudwego/kitex/client"
	xds2 "github.com/cloudwego/kitex/pkg/xds"
	"github.com/kitex-contrib/xds"
	"github.com/kitex-contrib/xds/xdssuite"
	"github.com/cloudwego/kitex-proxyless-test/service/codec/thrift/kitex_gen/proxyless/greetservice"
)

func main() {
	// initialize xds module
	err := xds.Init()
	if err != nil {
		return
	}
	
	// initialize the client
	cli, err := greetservice.NewClient(
		destService,
		client.WithXDSSuite(xds2.ClientSuite{
			RouterMiddleware: xdssuite.NewXDSRouterMiddleware(),
			Resolver:         xdssuite.NewXDSResolver(),
		}),
	)
	
	req := &proxyless.HelloRequest{Message: "Hello!"}
	resp, err := c.cli.SayHello1(
		ctx, 
		req, 
	) 
}
```

Detailed examples can be found here [kitex-proxyless-example](https://github.com/cloudwego/kitex-examples/tree/main/proxyless).

## Limitation
### mTLS
mTLS is not supported for now. Please disable mTLS via configuring PeerAuthentication.

```
apiVersion: "security.istio.io/v1beta1"
kind: "PeerAuthentication"
metadata:
  name: "default"
  namespace: {your_namespace}
spec:
  mtls:
    mode: DISABLE
``` 

### Limited support for Service Governance
Current version only support Service Discovery, Traffic route, Rate Limit, Timeout Configuration via xDS on the client-side and circuit-breaking. 

Other features supported via xDS, including Load Balancing and Retry etc, will be added in the future.

## Compatibility
This package is only tested under Istio1.13.3.

maintained by: [ppzqh](https://github.com/ppzqh)

## Dependencies
Kitex >= v0.4.0

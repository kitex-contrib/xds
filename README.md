# xDS support
[xDS](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol) is a set of discovery services that enables date-plane to discover various dynamic resources via querying from the management servers.

This project adds xDS support for Kitex and enables Kitex to perform in Proxyless mode. For more details of the design, please refer to the [proposal](https://github.com/cloudwego/kitex/issues/461).

## Feature

* Service Discovery:
* Traffic Route: only support `exact` match for `header` and `method`
	* [HTTP route configuration](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/http/http_routing#arch-overview-http-routing): configure via VirtualService
	* [ThriftProxy](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/thrift_proxy/v3/thrift_proxy.proto): configure via patching EnvoyFilter
* Timeout Configuration:
	* 	Configuration inside HTTP route configuration: configure via VirtualService

## Usage
### xDS module
To enable xDS mode in Kitex, we should invoke `xds.Init()` to initialize the xds module, including the `xdsResourceManager` and `xdsClient`.

#### Bootstrap
The xdsClient is responsible for the interaction with the xDS Server (i.e. Istio). It needs some environment variables for initialization, which need to be set inside the `spec.containers.env` of the Kubernetes Manifest file in YAML format.

* `POD_NAMESPACE`: the namespace of the current service.
*  `POD_NAME`: the name of this pod.
*  `INSTANCE_IP`: the ip of this pod.

Add the following part to the definition of your container.

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
```

### Client-side

For now, we only provide the support on the client-side. To use a xds-enabled Kitex client, you should specify `destService` using the URL of your target service and add one option `WithXDSSuite`.

```
client.WithXDSSuite(
	xdssuite.NewXDSRouterMiddleware(),
	xdssuite.NewXDSResolver(),
),
```

The URL of the target service should be in the format, which follows the format in [Kubernetes](https://kubernetes.io/):

```
<service-name>.<namespace>.svc.cluster.local:<service-port>
```

#### Traffic route based on Tag Match

We can define traffic route configuration via [VirtualService](https://istio.io/latest/docs/reference/config/networking/virtual-service/) in Istio.

```
http:
  - name: "route-based-on-tags"
    match:
      - headers:
          stage:
            exact: "canary"
```

To match the rule defined in VirtualService, we can use `client.WithTag(key, val string)` or `callopt.WithTag(key, val string)`to specify the tags, which will be used to match the rules.

* Set key and value to be "stage" and "canary" to match the above rule defined in VirtualService.
```
client.WithTag("stage", "canary")
callopt.WithTag("stage", "canary")
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

Detailed examples can be found here [kitex-proxyless-example](https://github.com/ppzqh/kitex-proxyless-test/tree/example).

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

### More support for Service Governance
Current version only support Service Discovery, Traffic route and Timeout Configuration via xDS on the client-side. 

Other features supported via xDS, including Load Balancing, Rate Limit and Retry etc, will be added in the future.

## Compatibility
This package is only tested under Istio1.13.3.

maintained by: [ppzqh](https://github.com/ppzqh)

## Dependencies
Kitex >= v0.4.0

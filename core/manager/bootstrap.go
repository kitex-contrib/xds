/*
 * Copyright 2022 CloudWeGo Authors
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

package manager

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	PodNamespace   = "POD_NAMESPACE"
	PodName        = "POD_NAME"
	MetaNamespace  = "NAMESPACE"
	InstanceIP     = "INSTANCE_IP"
	KitexXdsDomain = "KITEX_XDS_DOMAIN"
	// use json to marshal it.
	KitexXdsMetas = "KITEX_XDS_METAS"

	IstioAddrEnvKey        = "KITEX_XDS_ISTIO_ADDR"
	IstioServiceNameEnvKey = "KITEX_XDS_ISTIO_SERVICE_NAME"
	IstioAuthEnvKey        = "KITEX_XDS_ISTIO_AUTH"
	IstioVersion           = "ISTIO_VERSION"
	IstioMetaInstanceIPs   = "INSTANCE_IPS"
)

var (
	IstiodAddr      = "istiod.istio-system.svc:15010"
	IstiodSvrName   = "istiod.istio-system.svc"
	IstioAuthEnable = false
)

func init() {
	istiodAddr := os.Getenv(IstioAddrEnvKey)
	if istiodAddr != "" {
		IstiodAddr = istiodAddr
	}

	istiodSvrName := os.Getenv(IstioServiceNameEnvKey)
	if istiodSvrName != "" {
		IstiodSvrName = istiodSvrName
	}

	istiodAuthEnable := os.Getenv(IstioAuthEnvKey)
	if istiodAuthEnable == "true" {
		IstioAuthEnable = true
	}
}

type BootstrapConfig struct {
	// The namespace to make up fqdn.
	// Use POD_NAMESPACE default, the meta namespace will override that if set.
	configNamespace string
	nodeDomain      string
	node            *v3core.Node
	xdsSvrCfg       *XDSServerConfig
}

type XDSServerConfig struct {
	SvrName         string        // The name of the xDS server
	SvrAddr         string        // The address of the xDS server
	XDSAuth         bool          // If this xDS enable the authentication of xDS stream
	NDSNotRequired  bool          // required by default for Istio
	LDSNotRequired  bool          // required by default for Istio
	FetchXDSTimeout time.Duration // timeout for fecth xds, default to 1s
}

// GetFetchXDSTimeout get timeout.
func (xsc XDSServerConfig) GetFetchXDSTimeout() time.Duration {
	if xsc.FetchXDSTimeout == 0 {
		return defaultXDSFetchTimeout
	}
	return xsc.FetchXDSTimeout
}

func parseMetaEnvs(envs, istioVersion, podIP string) *structpb.Struct {
	defaultMeta := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			IstioVersion: {
				Kind: &structpb.Value_StringValue{StringValue: istioVersion},
			},
		},
	}
	if len(envs) == 0 {
		return defaultMeta
	}

	pbmeta := &structpb.Struct{
		Fields: map[string]*structpb.Value{},
	}
	err := protojson.Unmarshal([]byte(envs), pbmeta)
	if err != nil {
		klog.Warnf("[Kitex] XDS meta info is invalid %s, error %v", envs, err)
		return defaultMeta
	}
	if ips, ok := pbmeta.Fields[IstioMetaInstanceIPs]; ok {
		existips := ips.GetStringValue()
		if existips == "" {
			existips = podIP
		} else if !strings.Contains(existips, podIP) {
			existips = existips + "," + podIP
		}
		pbmeta.Fields[IstioMetaInstanceIPs] = &structpb.Value{
			Kind: &structpb.Value_StringValue{StringValue: existips},
		}
	}
	return pbmeta
}

func nodeId(podIP, podName, namespace, nodeDomain string) string {
	//"sidecar~" + podIP + "~" + podName + "." + namespace + "~" + namespace + ".svc." + domain,
	return fmt.Sprintf("sidecar~%s~%s.%s~%s.svc.%s", podIP, podName, namespace, namespace, nodeDomain)
}

// tryExpandFQDN try expand fully qualified domain name.
func (bc *BootstrapConfig) tryExpandFQDN(host string) string {
	// The kubernetes services following the <serviceName>.<ns>.svc.<suffix> naming convention
	// and that share a suffix with the domain. If it already been expanded, ignore it.
	if strings.Contains(host, ".svc.") {
		return host
	}

	var b strings.Builder
	b.Grow(len(host) + len(bc.configNamespace) + len(bc.nodeDomain) + 10)
	b.WriteString(host)

	// the regex for Kubernetes service is [a-z]([-a-z0-9]*[a-z0-9])?, it should not contains `.`, so we can split the host
	// by `.`
	// if the host not contains namespace.
	parts := strings.Split(host, ".")
	switch len(parts) {
	case 1:
		b.WriteString(".")
		b.WriteString(bc.configNamespace)
		fallthrough
	case 2:
		b.WriteString(".svc.")
		b.WriteString(bc.nodeDomain)
	case 3:
		if parts[2] == "svc" {
			b.WriteString(".")
			b.WriteString(bc.nodeDomain)
		}
	default:
		// if the host contains more than 4 parts, try to add all the info.
		b.WriteString(".")
		b.WriteString(bc.configNamespace)
		b.WriteString(".svc.")
		b.WriteString(bc.nodeDomain)
	}

	return b.String()
}

// newBootstrapConfig constructs the bootstrapConfig
func newBootstrapConfig(config *XDSServerConfig) (*BootstrapConfig, error) {
	// Get info from env
	// Info needed for construct the nodeID
	namespace := os.Getenv(PodNamespace)
	if namespace == "" {
		return nil, fmt.Errorf("[XDS] Bootstrap, POD_NAMESPACE is not set in env")
	}
	podName := os.Getenv(PodName)
	if podName == "" {
		return nil, fmt.Errorf("[XDS] Bootstrap, POD_NAME is not set in env")
	}
	podIP := os.Getenv(InstanceIP)
	if podIP == "" {
		return nil, fmt.Errorf("[XDS] Bootstrap, INSTANCE_IP is not set in env")
	}
	if config.FetchXDSTimeout == 0 {
		config.FetchXDSTimeout = defaultXDSFetchTimeout
	}
	// specify the version of istio in case of the canary deployment of istiod
	istioVersion := os.Getenv(IstioVersion)
	nodeDomain := os.Getenv(KitexXdsDomain)
	if nodeDomain == "" {
		nodeDomain = "cluster.local"
	}

	bsConfig := &BootstrapConfig{
		nodeDomain:      nodeDomain,
		configNamespace: namespace,
		node: &v3core.Node{
			Id:       nodeId(podIP, podName, namespace, nodeDomain),
			Metadata: parseMetaEnvs(os.Getenv(KitexXdsMetas), istioVersion, podIP),
		},
		xdsSvrCfg: config,
	}

	// the priority of NAMESPACE in metadata is higher than POD_NAMESPACE.
	// ref: https://github.com/istio/istio/blob/30446a7b88aba4a0fcd5f71bae8d397a874e846f/pilot/pkg/model/context.go#L1024
	if field, ok := bsConfig.node.Metadata.Fields[MetaNamespace]; ok {
		if val := field.GetStringValue(); val != "" {
			bsConfig.configNamespace = val
		}
	}
	return bsConfig, nil
}

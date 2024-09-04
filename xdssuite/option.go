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

package xdssuite

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/xds"
	"github.com/cloudwego/kitex/server"
)

type routerMetaExtractor func(context.Context) map[string]string

// Options for xds suite
type Options struct {
	routerMetaExtractor       routerMetaExtractor // use metainfo.GetAllValues by default.
	servicePort               uint32
	matchRetryMethod          bool
	enableServiceCircuitBreak bool
}

func (o *Options) Apply(opts []Option) {
	for _, op := range opts {
		op.F(o)
	}
}

type Option struct {
	F func(o *Options)
}

func NewOptions(opts []Option) *Options {
	o := &Options{
		routerMetaExtractor: func(ctx context.Context) map[string]string {
			return metainfo.GetAllValues(ctx)
		},
	}
	o.Apply(opts)
	return o
}

// WithServicePort configures the service port, used for rate limit.
func WithServicePort(port uint32) Option {
	return Option{
		F: func(o *Options) {
			o.servicePort = port
		},
	}
}

// WithMatchRetryMethod configures the flag of matchRetryMethod
func WithMatchRetryMethod(match bool) Option {
	return Option{
		F: func(o *Options) {
			o.matchRetryMethod = match
		},
	}
}

// WithServiceCircuitBreak if enable service dimension circuitbreak
func WithServiceCircuitBreak(enable bool) Option {
	return Option{
		F: func(o *Options) {
			o.enableServiceCircuitBreak = enable
		},
	}
}

// WithRouterMetaExtractor configures the extractor for metadata
func WithRouterMetaExtractor(routerMetaExtractor routerMetaExtractor) Option {
	return Option{
		F: func(o *Options) {
			o.routerMetaExtractor = routerMetaExtractor
		},
	}
}

type clientSuite struct {
	cOpts []client.Option
}

func (c *clientSuite) Options() []client.Option {
	return c.cOpts
}

// NewClientSuite client suite for xds handler
func NewClientSuite(opts ...Option) client.Option {
	cOpts := []client.Option{
		client.WithXDSSuite(xds.ClientSuite{
			RouterMiddleware: NewXDSRouterMiddleware(opts...),
			Resolver:         NewXDSResolver(),
		}),
		NewCircuitBreaker(opts...),
		NewRetryPolicy(opts...),
	}
	return client.WithSuite(&clientSuite{cOpts})
}

type serverSuite struct {
	cOpts []server.Option
}

func (s *serverSuite) Options() []server.Option {
	return s.cOpts
}

// NewServerSuite server suite for xds handler
func NewServerSuite(opts ...Option) server.Option {
	cOpts := []server.Option{
		NewLimiter(opts...),
	}
	return server.WithSuite(&serverSuite{cOpts})
}

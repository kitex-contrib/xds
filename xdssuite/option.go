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
)

type routerMetaExtractor func(context.Context) map[string]string

// Options for xds suite
type Options struct {
	routerMetaExtractor routerMetaExtractor // use metainfo.GetAllValues by default.
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

// WithRouterMetaExtractor configures the extractor for metadata
func WithRouterMetaExtractor(routerMetaExtractor routerMetaExtractor) Option {
	return Option{
		F: func(o *Options) {
			o.routerMetaExtractor = routerMetaExtractor
		},
	}
}

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

type metadataExtract func(context.Context) map[string]string

// Options for xds suite
type Options struct {
	metadataExtract metadataExtract
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
		metadataExtract: func(ctx context.Context) map[string]string {
			return metainfo.GetAllValues(ctx)
		},
	}
	o.Apply(opts)
	return o
}

// WithMetadataExtract configures the extractor for metadata
func WithMetadataExtract(metadataExtract metadataExtract) Option {
	return Option{
		F: func(o *Options) {
			o.metadataExtract = metadataExtract
		},
	}
}

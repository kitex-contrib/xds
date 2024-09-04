/*
 * Copyright 2024 CloudWeGo Authors
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
	"math"
	"testing"

	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/stretchr/testify/assert"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

func TestLimiter(t *testing.T) {
	l := &limiter{
		opt:  &limit.Option{},
		port: 80,
	}

	l.opt.UpdateControl = func(u limit.Updater) {
		u.UpdateLimit(l.opt)
		l.updater.Store(u)
	}

	l.listenerUpdater(map[string]xdsresource.Resource{
		xdsresource.ReservedLdsResourceName: &xdsresource.ListenerResource{
			NetworkFilters: []*xdsresource.NetworkFilter{
				{
					RoutePort: 80,
					InlineRouteConfig: &xdsresource.RouteConfigResource{
						TokensPerFill: 100,
					},
				},
			},
		},
	})
	assert.Equal(t, l.opt.MaxQPS, 100)
	assert.Equal(t, l.opt.MaxConnections, math.MaxInt)

	l.listenerUpdater(map[string]xdsresource.Resource{
		"123": &xdsresource.ListenerResource{
			NetworkFilters: []*xdsresource.NetworkFilter{
				{
					RoutePort: 80,
					InlineRouteConfig: &xdsresource.RouteConfigResource{
						TokensPerFill: 100,
					},
				},
			},
		},
	})
	assert.Equal(t, l.opt.MaxQPS, math.MaxInt)
	assert.Equal(t, l.opt.MaxConnections, math.MaxInt)

	l.listenerUpdater(map[string]xdsresource.Resource{
		xdsresource.ReservedLdsResourceName: &xdsresource.ListenerResource{
			NetworkFilters: []*xdsresource.NetworkFilter{
				{
					RoutePort: 80,
					InlineRouteConfig: &xdsresource.RouteConfigResource{
						TokensPerFill: 1000,
					},
				},
			},
		},
	})
	assert.Equal(t, l.opt.MaxQPS, 1000)
	assert.Equal(t, l.opt.MaxConnections, math.MaxInt)

	l.listenerUpdater(map[string]xdsresource.Resource{
		xdsresource.ReservedLdsResourceName: &xdsresource.ListenerResource{
			NetworkFilters: []*xdsresource.NetworkFilter{
				{
					RoutePort: 0,
					InlineRouteConfig: &xdsresource.RouteConfigResource{
						TokensPerFill: 999,
					},
				},
			},
		},
	})
	assert.Equal(t, l.opt.MaxQPS, 999)
	assert.Equal(t, l.opt.MaxConnections, math.MaxInt)

	l.listenerUpdater(map[string]xdsresource.Resource{
		xdsresource.ReservedLdsResourceName: &xdsresource.ListenerResource{
			NetworkFilters: []*xdsresource.NetworkFilter{
				{
					RoutePort: 8080,
					InlineRouteConfig: &xdsresource.RouteConfigResource{
						TokensPerFill: 999,
					},
				},
			},
		},
	})
	assert.Equal(t, l.opt.MaxQPS, math.MaxInt)
	assert.Equal(t, l.opt.MaxConnections, math.MaxInt)
}

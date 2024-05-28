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
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/server"

	"github.com/kitex-contrib/xds/core/xdsresource"
)

func setLimitOption(token uint32) *limit.Option {
	maxQPS := int(token)
	// if the token is zero, set the value to Max to disable the limiter
	if maxQPS == 0 {
		maxQPS = math.MaxInt
	}
	return &limit.Option{
		MaxQPS: maxQPS,
		// TODO: there is no conresponse config in xDS, disable it default.
		MaxConnections: math.MaxInt,
	}
}

func getLimiterPolicy(up map[string]xdsresource.Resource) map[uint32]uint32 {
	val, ok := up[xdsresource.ReservedLdsResourceName]
	if !ok {
		return nil
	}
	lds, ok := val.(*xdsresource.ListenerResource)
	if !ok {
		return nil
	}
	if lds == nil {
		return nil
	}
	maxTokens := make(map[uint32]uint32)
	for _, lis := range lds.NetworkFilters {
		if lis.InlineRouteConfig != nil {
			maxTokens[lis.RoutePort] = lis.InlineRouteConfig.MaxTokens
		}
	}
	return maxTokens
}

type limiter struct {
	updater atomic.Value
	opt     *limit.Option
	port    uint32
}

func (l *limiter) updateHandler(changed *limit.Option) {
	if changed == nil {
		return
	}
	l.opt.MaxConnections = changed.MaxConnections
	l.opt.MaxQPS = changed.MaxQPS
	u := l.updater.Load()
	if u == nil {
		klog.Warnf("[xds] server limiter config failed as the updater is empty")
		return
	}
	up, ok := u.(limit.Updater)
	if !ok {
		return
	}
	if !up.UpdateLimit(l.opt) {
		klog.Warnf("[xds] server limiter config: data %s may do not take affect", changed)
	}
}

func (l *limiter) listenerUpdater(res map[string]xdsresource.Resource) {
	tokens := getLimiterPolicy(res)
	klog.Debugf("[xds]getLimiterPolicy info: %v", tokens)
	var opt *limit.Option
	if mt, ok := tokens[l.port]; ok {
		opt = setLimitOption(mt)
	} else if mt, ok := tokens[xdsresource.DefaultServerPort]; ok {
		// if not find the port, use the default server port
		opt = setLimitOption(mt)
	} else {
		opt = setLimitOption(mt)
	}
	l.updateHandler(opt)
}

// NewLimiter creates a server limiter
func NewLimiter(opts ...Option) server.Option {
	serverOpt := NewOptions(opts)
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}

	l := &limiter{
		opt:  &limit.Option{},
		port: serverOpt.servicePort,
	}

	l.opt.UpdateControl = func(u limit.Updater) {
		u.UpdateLimit(l.opt)
		l.updater.Store(u)
	}

	m.RegisterXDSUpdateHandler(xdsresource.ListenerType, l.listenerUpdater)
	return server.WithLimit(l.opt)
}

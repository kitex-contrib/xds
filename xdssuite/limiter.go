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
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/server"
)

// NewLimiter creates a server limiter
func NewLimiter(opts ...Option) server.Option {
	serverOpt := NewOptions(opts)
	var updater atomic.Value
	opt := &limit.Option{}
	opt.UpdateControl = func(u limit.Updater) {
		u.UpdateLimit(opt)
		updater.Store(u)
	}
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}
	m.RegisterLimiter(serverOpt.servicePort, func(chanegd *limit.Option) {
		if chanegd == nil {
			return
		}
		opt.MaxConnections = chanegd.MaxConnections
		opt.MaxQPS = chanegd.MaxQPS
		u := updater.Load()
		if u == nil {
			klog.Warnf("[xds] server limiter config failed as the updater is empty")
			return
		}
		up, ok := u.(limit.Updater)
		if !ok {
			return
		}
		if !up.UpdateLimit(opt) {
			klog.Warnf("[xds] server limiter config: data %s may do not take affect", chanegd)
		}
	})
	return server.WithLimit(opt)
}

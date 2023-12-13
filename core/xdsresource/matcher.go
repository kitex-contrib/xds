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

package xdsresource

import (
	"regexp"
	"strings"

	"github.com/cloudwego/kitex/pkg/klog"
	v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	v3matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
)

type PrefixMatcher string

func (pm PrefixMatcher) Match(other string) bool {
	return strings.HasPrefix(other, string(pm))
}

type ExactMatcher string

func (em ExactMatcher) Match(other string) bool {
	return string(em) == other
}

type RegexMatcher struct {
	re *regexp.Regexp
}

func (rm *RegexMatcher) Match(other string) bool {
	return rm.re.MatchString(other)
}

type Matcher interface {
	Match(string) bool
}

type Matchers map[string]Matcher

func (ms Matchers) Match(other map[string]string) bool {
	for key, m := range ms {
		if val, ok := other[key]; !ok || !m.Match(val) {
			return false
		}
	}
	return true
}

// BuildMatchers build matcher set from headers
func BuildMatchers(headers []*v3.HeaderMatcher) Matchers {
	ms := map[string]Matcher{}
	for _, header := range headers {
		switch hm := header.GetHeaderMatchSpecifier().(type) {
		case *v3.HeaderMatcher_StringMatch:
			switch p := hm.StringMatch.GetMatchPattern().(type) {
			case *v3matcher.StringMatcher_Exact:
				if p.Exact != "" {
					ms[header.Name] = ExactMatcher(p.Exact)
				}
			case *v3matcher.StringMatcher_Prefix:
				if p.Prefix != "" {
					ms[header.Name] = PrefixMatcher(p.Prefix)
				}
			case *v3matcher.StringMatcher_SafeRegex:
				// only support google re2
				if p.SafeRegex != nil && p.SafeRegex.Regex != "" {
					re2, err := regexp.Compile(p.SafeRegex.Regex)
					if err != nil {
						klog.Warnf("KITEX: [XDS] compile regexp %s failed when BuildMatchers,  err:", p.SafeRegex.Regex, err)
						continue
					}
					ms[header.Name] = &RegexMatcher{
						re: re2,
					}
				}
			}
		}
	}
	return ms
}

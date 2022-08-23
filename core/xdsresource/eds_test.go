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
	"reflect"
	"strconv"
	"testing"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshalEDSError(t *testing.T) {
	tests := []struct {
		name         string
		rawResources []*any.Any
		want         map[string]*EndpointsResource
		wantErr      bool
	}{
		{
			name:         "resource is nil",
			rawResources: nil,
			want:         map[string]*EndpointsResource{},
			wantErr:      false,
		},
		{
			name: "incorrect resource type url",
			rawResources: []*any.Any{
				{TypeUrl: ListenerTypeURL, Value: []byte{}},
			},
			want:    map[string]*EndpointsResource{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalEDS(tt.rawResources)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalEDS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalEDS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalEDSSuccess(t *testing.T) {
	// edsAddr := "127.0.0.1"
	// edsPort1, edsPort2 := 8080, 8081
	// edsWeight1, edsWeight2 := 50, 50
	rawResources := []*any.Any{
		MarshalAny(Endpoints1),
	}
	got, err := UnmarshalEDS(rawResources)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(got))
	cla := got[EndpointName1]
	assert.NotNil(t, cla)
	assert.Equal(t, 1, len(cla.Localities))
	assert.Equal(t, 2, len(cla.Localities[0].Endpoints))
	e1 := cla.Localities[0].Endpoints[0]
	assert.Equal(t, 50, int(e1.Weight()))
	assert.Equal(t, edsAddr+":"+strconv.Itoa(edsPort1), e1.Addr().String())
	e2 := cla.Localities[0].Endpoints[1]
	assert.Equal(t, 50, int(e2.Weight()))
	assert.Equal(t, edsAddr+":"+strconv.Itoa(edsPort2), e2.Addr().String())
}

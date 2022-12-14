package manager

import (
	"testing"

	"github.com/cloudwego/thriftgo/pkg/test"
)

func Test_getTLSConfig(t *testing.T) {
	cfg, err := GetTLSConfig()
	test.Assert(t, err == nil, "GetTLSConfig failed: %v", err)
	test.Assert(t, cfg != nil, "GetTLSConfig failed: %v", err)
}

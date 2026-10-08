//go:build !clickhouse

package clickhouse

import (
	"fmt"

	"github.com/agentjetson/voice-query-service/internal/config"
)

// newRealBackend is a stub when the clickhouse build tag is not set.
func newRealBackend(cfg *config.Config) (queryBackend, error) {
	return nil, fmt.Errorf("ClickHouse driver not compiled in; rebuild with -tags clickhouse after: go get github.com/ClickHouse/clickhouse-go/v2@v2.30.0 (or set DEMO_MODE=true)")
}

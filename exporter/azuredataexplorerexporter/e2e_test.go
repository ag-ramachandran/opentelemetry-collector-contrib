// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"os"
	"testing"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/data/table"
	"github.com/Azure/azure-kusto-go/kusto/data/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestCreateTracesExporterE2E(t *testing.T) {
	config := getConfig(t)
	// Create an exporter
	f := NewFactory()
	exp, err := f.CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), config)
	require.NoError(t, err)
	err = exp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	// create some traces
	td, tID, attrs := createTraces()
	err = exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	kustoDefinitions := make(kusto.ParamTypes)
	kustoParameters := make(kusto.QueryValues)
	kustoDefinitions["TID"] = kusto.ParamType{Type: types.String}
	kustoParameters["TID"] = tID
	// Statements
	traceStmt := kusto.NewStmt("OTELTraces | where TraceID == TID")
	traceStmt = traceStmt.MustDefinitions(kusto.NewDefinitions().Must(kustoDefinitions)).MustParameters(kusto.NewParameters().Must(kustoParameters))

	// Query using our singleNodeStmt, variable substituting for ParamNodeId
	iter, err := createClientAndExecuteQuery(t, *config, traceStmt)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	// Validate the results
	recs := []AdxTrace{}
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
			rec := AdxTrace{}
			if err = row.ToStruct(&rec); err != nil {
				return err
			}
			recs = append(recs, rec)
			return nil
		},
	)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	// Validate all attributes
	for i := 0; i < len(recs); i++ {
		assert.Equal(t, tID, recs[i].TraceID)
		assert.Equal(t, "0102030405060708", recs[i].SpanID)
		assert.Equal(t, "", recs[i].ParentID)
		assert.Equal(t, "UnitTestTraces", recs[i].SpanName)
		assert.Equal(t, "STATUS_CODE_UNSET", recs[i].SpanStatus)
		assert.Equal(t, "SPAN_KIND_UNSPECIFIED", recs[i].SpanKind)
		assert.Equal(t, "1970-01-01 00:00:00.0000000", recs[i].StartTime)
		assert.Equal(t, "1970-01-01 00:00:00.0000000", recs[i].EndTime)
		assert.Equal(t, attrs, recs[i].TraceAttributes)
	}
}

func getConfig(t *testing.T) *Config {
	if os.Getenv("ClusterURI") == "" || os.Getenv("ApplicationID") == "" || os.Getenv("ApplicationKey") == "" || os.Getenv("TenantID") == "" || os.Getenv("Database") == "" {
		t.Skip("Environment variables ClusterURI/AppId/ApplicationKey/TenantID/Database is/are empty.Tests will be skipped")
	}
	clusterURI := os.Getenv("ClusterURI")
	appID := os.Getenv("ApplicationID")
	appKey := os.Getenv("ApplicationKey")
	tenantID := os.Getenv("TenantID")
	database := os.Getenv("Database")

	return &Config{
		ClusterURI:     clusterURI,
		ApplicationID:  appID,
		ApplicationKey: configopaque.String(appKey),
		TenantID:       tenantID,
		Database:       database,
		IngestionType:  "managed",
		MetricTable:    "OTELMetrics",
		LogTable:       "OTELLogs",
		TraceTable:     "OTELTraces",
	}
}

func createTraces() (ptrace.Traces, string, map[string]interface{}) {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("UnitTestTraces")
	attrs := map[string]interface{}{
		"k0": "v0",
		"k1": "v1",
	}
	// This error can be ignored. Just optional attribute addition. This will fail assertion in that case
	_ = span.Attributes().FromRaw(attrs)
	span.SetStartTimestamp(pcommon.Timestamp(10))
	span.SetEndTimestamp(pcommon.Timestamp(20))
	// Create a random TraceId
	tID := uuid.New().String()
	var traceArray [16]byte
	copy(traceArray[:], tID)
	span.SetTraceID(pcommon.TraceID(traceArray))
	// For now hardcode the span 1d
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	return td, tID, attrs
}

func createClientAndExecuteQuery(t *testing.T, config Config, stmt kusto.Stmt) (*kusto.RowIterator, error) {
	kcsb := kusto.NewConnectionStringBuilder(config.ClusterURI).WithAadAppKey(config.ApplicationID, string(config.ApplicationKey), config.TenantID)
	client, kerr := kusto.New(kcsb)
	// The client should be created
	if kerr != nil {
		assert.Fail(t, kerr.Error())
	}
	defer client.Close()
	// Query using our singleNodeStmt, variable substituting for ParamNodeId
	return client.Query(
		context.Background(),
		config.Database,
		stmt,
	)
}

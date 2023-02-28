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
	"fmt"
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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	appID                 = "APP_ID"
	appKey                = "APP_KEY"
	clusterURI            = "CLUSTER_URI"
	debugLevel            = "Debug"
	epochTimeString       = "1970-01-01 00:00:00.0000000Z"
	logStatement          = "A unit test log with trace"
	logTable              = "OTELLogs"
	logValidationQuery    = "TBL | extend Timestamp=tostring(Timestamp),ObservedTimestamp=tostring(ObservedTimestamp) | where TraceID == TID"
	metricDescription     = "A unit-test gauge metric"
	metricName            = "test_gauge"
	metricTable           = "OTELMetrics"
	metricUnit            = "%"
	metricValidationQuery = "TBL | extend Timestamp=tostring(Timestamp) | where MetricName == TID"
	metricValue           = 42.42
	skipMessage           = "Environment variables CLUSTER_URI/APP_ID/APP_KEY/AZURE_TENANT_ID/OTEL_DB is/are empty.Tests will be skipped"
	spanID                = "0102030405060708"
	spanName              = "UnitTestTraces"
	tenantID              = "AZURE_TENANT_ID"
	traceTable            = "OTELTraces"
	traceValidationQuery  = "TBL | extend StartTime=tostring(StartTime),EndTime=tostring(EndTime) | where TraceID == TID"
)

// E2E tests while sending the trace data through the exporter
func TestCreateTracesExporterE2E(t *testing.T) {
	t.Parallel()
	config, isValid := getConfig()
	if !isValid {
		t.Skip(skipMessage)
	}
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
	kustoDefinitions, kustoParameters := prepareQuery(tID, traceTable)
	// Statements
	traceStmt := kusto.NewStmt(traceValidationQuery)
	traceStmt = traceStmt.MustDefinitions(kusto.NewDefinitions().Must(kustoDefinitions)).MustParameters(kusto.NewParameters().Must(kustoParameters))
	// Query using our trace table for TraceID
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
		assert.Equal(t, spanID, recs[i].SpanID)
		assert.Equal(t, "", recs[i].ParentID)
		assert.Equal(t, spanName, recs[i].SpanName)
		assert.Equal(t, "STATUS_CODE_UNSET", recs[i].SpanStatus)
		assert.Equal(t, "SPAN_KIND_UNSPECIFIED", recs[i].SpanKind)
		assert.Equal(t, epochTimeString, recs[i].StartTime)
		assert.Equal(t, epochTimeString, recs[i].EndTime)
		assert.Equal(t, attrs, recs[i].TraceAttributes)
	}
	t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })
}

// E2E tests while sending the logs data through the exporter
func TestCreateLogsExporterE2E(t *testing.T) {
	t.Parallel()
	config, isValid := getConfig()
	if !isValid {
		t.Skip(skipMessage)
	}
	// Create an exporter
	f := NewFactory()
	exp, err := f.CreateLogsExporter(context.Background(), exportertest.NewNopCreateSettings(), config)
	require.NoError(t, err)
	err = exp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	// create some traces
	ld, tID, attrs := createLogs()
	err = exp.ConsumeLogs(context.Background(), ld)
	require.NoError(t, err)
	kustoDefinitions, kustoParameters := prepareQuery(tID, logTable)
	// Statements
	traceStmt := kusto.NewStmt(logValidationQuery)
	traceStmt = traceStmt.MustDefinitions(kusto.NewDefinitions().Must(kustoDefinitions)).MustParameters(kusto.NewParameters().Must(kustoParameters))
	// Query using our logs table for TraceID
	iter, err := createClientAndExecuteQuery(t, *config, traceStmt)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	// Validate the results
	recs := []AdxLog{}
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
			rec := AdxLog{}
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
		crec := recs[i]
		assert.Equal(t, tID, crec.TraceID)
		assert.Equal(t, spanID, crec.SpanID)
		assert.Equal(t, epochTimeString, crec.ObservedTimestamp)
		assert.Equal(t, epochTimeString, crec.Timestamp)
		assert.Equal(t, attrs, crec.LogsAttributes)
		assert.Equal(t, int32(5) /*plog.SeverityNumberDebug*/, crec.SeverityNumber)
		assert.Equal(t, debugLevel, crec.SeverityText)
		assert.Equal(t, attrs, crec.LogsAttributes)
		assert.Equal(t, logStatement, crec.Body)
	}
	t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })
}

// E2E tests while sending the metrics data through the exporter
func TestCreateMetricsExporterE2E(t *testing.T) {
	t.Parallel()
	config, isValid := getConfig()
	if !isValid {
		t.Skip(skipMessage)
	}
	// Create an exporter
	f := NewFactory()
	exp, err := f.CreateMetricsExporter(context.Background(), exportertest.NewNopCreateSettings(), config)
	require.NoError(t, err)
	err = exp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	// create some traces
	md, attrs, metricName := createMetrics()
	err = exp.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	kustoDefinitions, kustoParameters := prepareQuery(metricName, metricTable)
	// Statements
	traceStmt := kusto.NewStmt(metricValidationQuery)
	traceStmt = traceStmt.MustDefinitions(kusto.NewDefinitions().Must(kustoDefinitions)).MustParameters(kusto.NewParameters().Must(kustoParameters))
	// Query using our logs table for TraceID
	iter, err := createClientAndExecuteQuery(t, *config, traceStmt)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	// Validate the results
	recs := []AdxMetric{}
	err = iter.DoOnRowOrError(
		func(row *table.Row, e *errors.Error) error {
			if e != nil {
				return e
			}
			rec := AdxMetric{}
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
		crec := recs[i]
		assert.Equal(t, metricName, crec.MetricName)
		assert.Equal(t, float64(metricValue), crec.MetricValue)
		assert.Equal(t, attrs, crec.MetricAttributes)
		assert.Equal(t, metricDescription, crec.MetricDescription)
		assert.Equal(t, metricUnit, crec.MetricUnit)
		assert.Equal(t, epochTimeString, crec.Timestamp)
	}
	t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })
}

func prepareQuery(tID string, table string) (kusto.ParamTypes, kusto.QueryValues) {
	kustoDefinitions := make(kusto.ParamTypes)
	kustoParameters := make(kusto.QueryValues)
	kustoDefinitions["TID"] = kusto.ParamType{Type: types.String}
	kustoParameters["TID"] = tID
	kustoDefinitions["TBL"] = kusto.ParamType{Type: types.String}
	kustoParameters["TBL"] = table
	return kustoDefinitions, kustoParameters
}

func getConfig() (*Config, bool) {
	if os.Getenv(clusterURI) == "" || os.Getenv(appID) == "" || os.Getenv(appKey) == "" || os.Getenv(tenantID) == "" || os.Getenv(otelDb) == "" {
		return nil, false
	}
	clusterURI := os.Getenv(clusterURI)
	clientID := os.Getenv(appID)
	appKey := os.Getenv(appKey)
	tenantID := os.Getenv(tenantID)
	database := os.Getenv(otelDb)

	return &Config{
		ClusterURI:     clusterURI,
		ApplicationID:  clientID,
		ApplicationKey: configopaque.String(appKey),
		TenantID:       tenantID,
		Database:       database,
		IngestionType:  "managed",
		MetricTable:    metricTable,
		LogTable:       logTable,
		TraceTable:     traceTable,
	}, true
}

func createTraces() (ptrace.Traces, string, map[string]interface{}) {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(spanName)
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
	var spanArray [8]byte
	copy(spanArray[:], spanID)
	copy(traceArray[:], tID)
	span.SetTraceID(pcommon.TraceID(traceArray))
	// For now hardcode the span 1d
	span.SetSpanID(spanArray)
	return td, traceutil.TraceIDToHexOrEmptyString(span.TraceID()), attrs
}

func createLogs() (plog.Logs, string, map[string]interface{}) {
	testLogs := plog.NewLogs()
	tID := uuid.New().String()
	attrs := map[string]interface{}{
		"l0": "a0",
		"l1": "a1",
	}
	var traceArray [16]byte
	copy(traceArray[:], tID)
	var spanArray [8]byte
	copy(spanArray[:], spanID)
	rl := testLogs.ResourceLogs()
	sl := rl.AppendEmpty().ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr(logStatement)
	logRecord.SetTraceID(pcommon.TraceID(traceArray))
	logRecord.SetSpanID(spanArray)
	logRecord.SetSeverityText(debugLevel)
	logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
	//nolint:errcheck
	logRecord.Attributes().FromRaw(attrs)
	logRecord.SetTimestamp(pcommon.Timestamp(10))
	return testLogs, traceutil.TraceIDToHexOrEmptyString(logRecord.TraceID()), attrs
}

func createMetrics() (pmetric.Metrics, map[string]interface{}, string) {
	tm := pmetric.NewMetrics()
	tID := uuid.New().String()
	attrs := map[string]interface{}{
		"m1": "a0",
		"m2": "a1",
	}
	rm := tm.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	m := ilm.Metrics().AppendEmpty()
	metricNameGUID := fmt.Sprintf("%s-%s", metricName, tID)
	m.SetName(metricNameGUID)
	m.SetDescription(metricDescription)
	m.SetUnit(metricUnit)
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	//nolint:errcheck
	dp.Attributes().FromRaw(attrs)
	dp.SetDoubleValue(metricValue)
	return tm, attrs, metricNameGUID
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

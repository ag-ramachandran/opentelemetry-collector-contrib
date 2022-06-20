// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// adxDataProducer uses the ADX client to perform ingestion
type adxDataProducer struct {
	client        *kusto.Client       // client for logs , traces and metrics
	managedingest *ingest.Managed     // managed ingestion for  logs, traces and metrics
	queuedingest  *ingest.Ingestion   // queued ingestion for  logs, traces and metrics
	ingestoptions []ingest.FileOption // Options for the ingestion
	logger        *zap.Logger         // Loggers for tracing the flow
}

var nextline = []byte("\n")

// given the full metrics , extract each metric , resource attributes and scope attributes. Individual metric mapping is sent on to metricdata mapping
func (e *adxDataProducer) metricsDataPusher(ctx context.Context, metrics pmetric.Metrics) error {
	metricsbytearray := make([]byte, 0)
	transformedadxmetrics, err := rawMetricsToAdxMetrics(ctx, metrics, e.logger)
	if err != nil {
		e.logger.Error("Error transforming metrics to ADX metric format.", zap.Error(err))
		return err
	}
	// Since the transform succeeded ,  using the option for ingestion ingest the data into ADX
	for tm := 0; tm < len(transformedadxmetrics); tm++ {
		adxmetricjsonbytes, err := jsoniter.Marshal(transformedadxmetrics[tm])
		if err != nil {
			e.logger.Error("Error performing serialization of data.", zap.Error(err))
		}
		metricsbytearray = bytes.Join([][]byte{metricsbytearray, adxmetricjsonbytes}, nextline)
	}
	if len(metricsbytearray) != 0 {
		if err := e.ingestData(metricsbytearray); err != nil {
			return err
		}
	}
	metricsflushed := len(transformedadxmetrics)
	e.logger.Sugar().Infof("Flushing %d metrics to sink", metricsflushed)
	return nil
}

func (e *adxDataProducer) ingestData(b []byte) error {

	ingestreader := bytes.NewReader(b)

	// Either of the ingestion pilicy will be present according to the configuration provided, other will be nil
	if e.managedingest != nil {
		if _, err := e.managedingest.FromReader(context.Background(), ingestreader, e.ingestoptions...); err != nil {
			e.logger.Error("Error performing managed data ingestion.", zap.Error(err))
			return err
		}
	} else {
		if _, err := e.queuedingest.FromReader(context.Background(), ingestreader, e.ingestoptions...); err != nil {
			e.logger.Error("Error performing queued data ingestion.", zap.Error(err))
			return err
		}
	}
	return nil
}

func (e *adxDataProducer) logsDataPusher(ctx context.Context, logData plog.Logs) error {
	resourceLogs := logData.ResourceLogs()
	logsArray := make([]byte, 0)
	nextline := []byte("\n")
	for i := 0; i < resourceLogs.Len(); i++ {
		resource := resourceLogs.At(i)
		scopeLogs := resourceLogs.At(i).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			scope := scopeLogs.At(j)
			logs := scopeLogs.At(j).LogRecords()

			for k := 0; k < logs.Len(); k++ {
				logData := logs.At(k)
				transformedAdxLog := mapToAdxLog(resource.Resource(), scope.Scope(), logData, e.logger)
				adxLogJsonBytes, err := jsoniter.Marshal(transformedAdxLog)
				adxLogJsonBytes = append(adxLogJsonBytes, nextline...)
				if err != nil {
					e.logger.Error("Error performing serialization of data.", zap.Error(err))
				}
				logsArray = append(logsArray, adxLogJsonBytes...)

			}
			if len(logsArray) != 0 {
				if err := e.ingestData(logsArray); err != nil {
					return err
				}
				logsArray = logsArray[:0]
			}
		}
	}
	return nil
}

func (e *adxDataProducer) tracesDataPusher(ctx context.Context, traceData ptrace.Traces) error {
	resourceSpans := traceData.ResourceSpans()
	spanDataArray := make([]byte, 0)
	nextline := []byte("\n")
	for i := 0; i < resourceSpans.Len(); i++ {
		resource := resourceSpans.At(i)
		ScopeSpans := resourceSpans.At(i).ScopeSpans()
		for j := 0; j < ScopeSpans.Len(); j++ {
			scope := ScopeSpans.At(j)
			spans := ScopeSpans.At(j).Spans()

			for k := 0; k < spans.Len(); k++ {
				spanData := spans.At(k)
				transformedAdxTrace := mapToAdxTrace(resource.Resource(), scope.Scope(), spanData, e.logger)
				adxTraceJsonBytes, err := jsoniter.Marshal(transformedAdxTrace)
				adxTraceJsonBytes = append(adxTraceJsonBytes, nextline...)
				if err != nil {
					e.logger.Error("Error performing serialization of data.", zap.Error(err))
				}
				spanDataArray = append(spanDataArray, adxTraceJsonBytes...)

			}
			if len(spanDataArray) != 0 {
				if err := e.ingestData(spanDataArray); err != nil {
					return err
				}
				spanDataArray = spanDataArray[:0]
			}
		}
	}
	return nil
}

func (amp *adxDataProducer) Close(context.Context) error {

	var err error
	err = amp.managedingest.Close()
	err2 := amp.client.Close()
	if err == nil {
		err = err2
	} else {
		err = errors.GetCombinedError(err, err2)
	}
	return err
}

/*
Create a metric exporter. The metric exporter instantiates a client , creates the ingester and then sends data through it
*/
func newExporter(config *Config, logger *zap.Logger, telemetrydatatype int) (*adxDataProducer, error) {
	tablename := getTableName(config, telemetrydatatype)
	metricclient, err := buildAdxClient(config)

	if err != nil {
		return nil, err
	}

	var managedingest *ingest.Managed
	var queuedingest *ingest.Ingestion

	// The exporter could be configured to run in either modes. Using managedstreaming or batched queueing
	if strings.ToLower(config.IngestionType) == managedingesttype {
		mi, err := createManagedStreamingIngester(config, metricclient, tablename)
		if err != nil {
			return nil, err
		}

		managedingest = mi
		queuedingest = nil
		err = nil
	} else {
		qi, err := createQueuedIngester(config, metricclient, tablename)
		if err != nil {
			return nil, err
		}
		managedingest = nil
		queuedingest = qi
		err = nil
	}
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.JSON)
	// Expect that this mapping is already existent
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower(tablename)), ingest.JSON)
	return &adxDataProducer{
		client:        metricclient,
		managedingest: managedingest,
		queuedingest:  queuedingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}, nil

}

/**
Common functions that are used by all the 3 parts of OTEL , namely Traces , Logs and Metrics
*/

func buildAdxClient(config *Config) (*kusto.Client, error) {
	authorizer := kusto.Authorization{
		Config: auth.NewClientCredentialsConfig(config.ClientId,
			config.ClientSecret, config.TenantId),
	}
	client, err := kusto.New(config.ClusterName, authorizer)
	return client, err
}

// Depending on the table , create seperate ingesters
func createManagedStreamingIngester(config *Config, adxclient *kusto.Client, tablename string) (*ingest.Managed, error) {
	//ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower(config.RawMetricTable)), ingest.MultiJSON)
	ingester, err := ingest.NewManaged(adxclient, config.Database, tablename)
	return ingester, err
}

// A queued ingester in case that is provided as the config option
func createQueuedIngester(config *Config, adxclient *kusto.Client, tablename string) (*ingest.Ingestion, error) {
	//ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower(config.RawMetricTable)), ingest.MultiJSON)
	ingester, err := ingest.New(adxclient, config.Database, tablename)
	return ingester, err
}

func getTableName(config *Config, telemetrydatatype int) string {
	switch telemetrydatatype {
	case MetricsType:
		return config.RawMetricTable
	}
	return "unknown"
}

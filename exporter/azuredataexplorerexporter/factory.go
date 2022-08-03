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
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr            = "azuredataexplorer"
	managedIngestType  = "managed"
	queuedIngestTest   = "queued"
	otelDb             = "oteldb"
	defaultMetricTable = "OTELMetrics"
	defaultLogTable    = "OTELLogs"
	defaultTraceTable  = "OTELTraces"
	metricsType        = 1
	logsType           = 2
	tracesType         = 3
)

// Creates a factory for the ADX Exporter
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesExporter(createTracesExporter),
		component.WithMetricsExporter(createMetricsExporter),
		component.WithLogsExporter(createLogsExporter),
	)
}

// Create default configurations
func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		Database:         otelDb,
		MetricTable:      defaultMetricTable,
		LogTable:         defaultLogTable,
		TraceTable:       defaultTraceTable,
		IngestionType:    queuedIngestTest,
	}
}

func createMetricsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.MetricsExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	adxCfg := config.(*Config)
	setDefaultIngestionType(adxCfg, set.Logger)

	// call the common exporter function in baseexporter. This ensures that the client and the ingest
	// are initialized and the metrics struct are available for operations
	adp, err := newExporter(adxCfg, set.Logger, metricsType)

	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		adxCfg,
		set,
		adp.metricsDataPusher,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithShutdown(adp.Close))

	if err != nil {
		return nil, err
	}
	return exporter, nil
}

func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.TracesExporter, error) {
	adxCfg := config.(*Config)
	setDefaultIngestionType(adxCfg, set.Logger)

	// call the common exporter function in baseexporter. This ensures that the client and the ingest
	// are initialized and the metrics struct are available for operations
	adp, err := newExporter(adxCfg, set.Logger, tracesType)

	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewTracesExporter(
		adxCfg,
		set,
		adp.tracesDataPusher,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithShutdown(adp.Close))

	if err != nil {
		return nil, err
	}
	return exporter, nil
}

func createLogsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (exp component.LogsExporter, err error) {
	adxCfg := config.(*Config)
	setDefaultIngestionType(adxCfg, set.Logger)

	// call the common exporter function in baseexporter. This ensures that the client and the ingest
	// are initialized and the metrics struct are available for operations
	adp, err := newExporter(adxCfg, set.Logger, logsType)

	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewLogsExporter(
		adxCfg,
		set,
		adp.logsDataPusher,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithShutdown(adp.Close))

	if err != nil {
		return nil, err
	}
	return exporter, nil
}

func setDefaultIngestionType(config *Config, logger *zap.Logger) {
	// If ingestion type is not set , it falls back to queued ingestion.
	// This form of ingestion is always available on all clusters
	if config.IngestionType == "" {
		logger.Warn("Ingestion type is not set , will be defaulted to queued ingestion")
		config.IngestionType = queuedIngestTest
	}
}

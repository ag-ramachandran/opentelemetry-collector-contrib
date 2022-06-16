package azuredataexplorerexporter

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type AdxLog struct {
	Timestamp            string                 //The timestamp of the occurance. Formatted into string as RFC3339
	ObservedTimestamp    string                 //The timestamp of logs observed in opentelemetry collector.  Formatted into string as RFC3339
	TraceId              string                 //TraceId associated to the log
	SpanId               string                 //SpanId associated to the log
	SeverityText         string                 //The severity level of the log
	SeverityNumber       int32                  //The severity number associated to the log
	Body                 string                 //The body/Text of the log
	ResourceData         map[string]interface{} //JSON Resource attributes that can then be parsed.
	InstrumentationScope map[string]string      //Scope data for the log
	Attributes           map[string]interface{} //JSON attributes that can then be parsed.
}

var LogsSeverityNumberMapper = map[string]int32{
	"SEVERITY_NUMBER_TRACE":  1,
	"SEVERITY_NUMBER_TRACE2": 2,
	"SEVERITY_NUMBER_TRACE3": 3,
	"SEVERITY_NUMBER_TRACE4": 4,
	"SEVERITY_NUMBER_DEBUG":  5,
	"SEVERITY_NUMBER_DEBUG2": 6,
	"SEVERITY_NUMBER_DEBUG3": 7,
	"SEVERITY_NUMBER_DEBUG4": 8,
	"SEVERITY_NUMBER_INFO":   9,
	"SEVERITY_NUMBER_INFO2":  10,
	"SEVERITY_NUMBER_INFO3":  11,
	"SEVERITY_NUMBER_INFO4":  12,
	"SEVERITY_NUMBER_WARN":   13,
	"SEVERITY_NUMBER_WARN2":  14,
	"SEVERITY_NUMBER_WARN3":  15,
	"SEVERITY_NUMBER_WARN4":  16,
	"SEVERITY_NUMBER_ERROR":  17,
	"SEVERITY_NUMBER_ERROR2": 18,
	"SEVERITY_NUMBER_ERROR3": 19,
	"SEVERITY_NUMBER_ERROR4": 20,
	"SEVERITY_NUMBER_FATAL":  21,
	"SEVERITY_NUMBER_FATAL2": 22,
	"SEVERITY_NUMBER_FATAL3": 23,
	"SEVERITY_NUMBER_FATAL4": 24,
}

/*
Convert the plog to the type ADXLog, this matches the scheme in the Log table in the database
*/

func mapToAdxLog(resource pcommon.Resource, scope pcommon.InstrumentationScope, logData plog.LogRecord, logger *zap.Logger) *AdxLog {

	adxLog := &AdxLog{
		Timestamp:            logData.Timestamp().AsTime().Format(time.RFC3339),
		ObservedTimestamp:    logData.ObservedTimestamp().AsTime().Format(time.RFC3339),
		TraceId:              logData.TraceID().HexString(),
		SpanId:               logData.SpanID().HexString(),
		SeverityText:         logData.SeverityText(),
		SeverityNumber:       int32(logData.SeverityNumber()),
		Body:                 logData.Body().AsString(),
		ResourceData:         resource.Attributes().AsRaw(),
		InstrumentationScope: getScopeMap(scope),
		Attributes:           logData.Attributes().AsRaw(),
	}
	return adxLog
}

func getScopeMap(sc pcommon.InstrumentationScope) map[string]string {
	if sc.Name() != "" || sc.Version() != "" {
		return map[string]string{
			"name":    sc.Name(),
			"version": sc.Version(),
		}
	}
	return map[string]string{}
}

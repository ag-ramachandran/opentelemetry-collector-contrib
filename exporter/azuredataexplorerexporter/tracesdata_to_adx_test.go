package azuredataexplorerexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func Test_mapToAdxTrace(t *testing.T) {
	logger := zap.NewNop()
	tsUnix := time.Unix(time.Now().Unix(), time.Now().UnixNano())
	ts := pcommon.NewTimestampFromTime(tsUnix)
	tstr := ts.AsTime().Format(time.RFC3339)
	tmap := make(map[string]interface{})
	tmap["key"] = "value"
	tmap[hostKey] = testhost

	scpMap := map[string]string{
		"name":    "testscope",
		"version": "1.0",
	}

	spanId := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceId := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	tests := []struct {
		name             string
		spanDatafn       func() ptrace.Span
		ResourceFn       func() pcommon.Resource
		insScopeFn       func() pcommon.InstrumentationScope
		expectedAdxTrace []*AdxTrace
	}{
		{
			name: "valid",
			spanDatafn: func() ptrace.Span {

				span := ptrace.NewSpan()
				span.SetName("spanname")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				span.SetTraceID(pcommon.NewTraceID(traceId))
				span.SetSpanID(pcommon.NewSpanID(spanId))
				span.SetKind(ptrace.SpanKindConsumer)
				span.SetStartTimestamp(ts)
				span.SetEndTimestamp(ts)
				span.Attributes().InsertString("traceAttribKey", "traceAttribVal")

				return span
			},
			insScopeFn: func() pcommon.InstrumentationScope {
				return newScopeWithData()
			},
			expectedAdxTrace: []*AdxTrace{
				{
					TraceId:              "00000000000000000000000000000064",
					SpanId:               "0000000000000032",
					ParentId:             "",
					SpanName:             "spanname",
					SpanStatus:           "STATUS_CODE_UNSET",
					SpanKind:             "SPAN_KIND_SERVER",
					StartTime:            tstr,
					EndTime:              tstr,
					ResourceAttributes:   tmap,
					InstrumentationScope: scpMap,
					TraceAttributes:      newMapFromAttr(`{"traceAttribKey":"traceAttribVal"}`),
					Events:               getDummyEvents(),
					Links:                getDummyLinks(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces := tt.traceDataFn()

			cfg := tt.configFn()
			event := mapSpanToSplunkEvent(traces.ResourceSpans().At(0).Resource(), traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0), cfg, logger)
			require.NotNil(t, event)
			assert.Equal(t, tt.wantSplunkEvent, event)
		})
	}

}

func getDummyEvents() []*Event {
	return []*Event{}

}

func getDummyLinks() []*Link {
	return []*Link{}
}

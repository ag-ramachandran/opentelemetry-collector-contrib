// Copyright 2020, OpenTelemetry Authors
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func Test_mapToAdxTrace(t *testing.T) {
	epoch, _ := time.Parse("2006-01-02T15:04:05Z07:00", "1970-01-01T00:00:00Z")
	defaultTime := pcommon.NewTimestampFromTime(epoch).AsTime().Format(time.RFC3339)
	tmap := make(map[string]interface{})
	tmap["key"] = "value"
	tmap[hostkey] = testhost

	spanID := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceID := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	tests := []struct {
		name             string             // name of the test
		spanDatafn       func() ptrace.Span // function that generates the
		resourceFn       func() pcommon.Resource
		insScopeFn       func() pcommon.InstrumentationScope
		expectedAdxTrace *AdxTrace
	}{
		{
			name: "valid",
			spanDatafn: func() ptrace.Span {

				span := ptrace.NewSpan()
				span.SetName("spanname")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				span.SetTraceID(pcommon.NewTraceID(traceID))
				span.SetSpanID(pcommon.NewSpanID(spanID))
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(ts)
				span.SetEndTimestamp(ts)
				span.Attributes().InsertString("traceAttribKey", "traceAttribVal")

				return span
			},
			resourceFn: func() pcommon.Resource {
				return newDummyResource()
			},
			insScopeFn: newScopeWithData,
			expectedAdxTrace: &AdxTrace{
				TraceID:            "00000000000000000000000000000064",
				SpanID:             "0000000000000032",
				ParentID:           "",
				SpanName:           "spanname",
				SpanStatus:         "STATUS_CODE_UNSET",
				SpanKind:           "SPAN_KIND_SERVER",
				StartTime:          tstr,
				EndTime:            tstr,
				ResourceAttributes: tmap,
				TraceAttributes:    newMapFromAttr(`{"traceAttribKey":"traceAttribVal", "scope.name":"testscope", "scope.version":"1.0"}`),
				Events:             getEmptyEvents(),
				Links:              getEmptyLinks(),
			},
		}, {
			name: "No data",
			spanDatafn: func() ptrace.Span {

				span := ptrace.NewSpan()
				return span
			},
			resourceFn: pcommon.NewResource,
			insScopeFn: newScopeWithData,
			expectedAdxTrace: &AdxTrace{
				SpanStatus:         "STATUS_CODE_UNSET",
				SpanKind:           "SPAN_KIND_UNSPECIFIED",
				StartTime:          defaultTime,
				EndTime:            defaultTime,
				ResourceAttributes: newMapFromAttr(`{}`),
				TraceAttributes:    newMapFromAttr(`{"scope.name":"testscope", "scope.version":"1.0"}`),
				Events:             getEmptyEvents(),
				Links:              getEmptyLinks(),
			},
		}, {
			name: "with_events_links",
			spanDatafn: func() ptrace.Span {

				span := ptrace.NewSpan()
				span.SetName("spanname")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				span.SetTraceID(pcommon.NewTraceID(traceID))
				span.SetSpanID(pcommon.NewSpanID(spanID))
				span.SetKind(ptrace.SpanKindServer)
				span.SetStartTimestamp(ts)
				span.SetEndTimestamp(ts)
				span.Attributes().InsertString("traceAttribKey", "traceAttribVal")
				event := span.Events().AppendEmpty()
				event.SetName("eventName")
				event.SetTimestamp(ts)
				event.Attributes().InsertString("eventkey", "eventvalue")

				link := span.Links().AppendEmpty()
				link.SetSpanID(pcommon.NewSpanID(spanID))
				link.SetTraceID(pcommon.NewTraceID(traceID))
				link.SetTraceState(ptrace.TraceStateEmpty)

				return span
			},
			resourceFn: func() pcommon.Resource {
				return newDummyResource()
			},
			insScopeFn: newScopeWithData,
			expectedAdxTrace: &AdxTrace{
				TraceID:            "00000000000000000000000000000064",
				SpanID:             "0000000000000032",
				ParentID:           "",
				SpanName:           "spanname",
				SpanStatus:         "STATUS_CODE_UNSET",
				SpanKind:           "SPAN_KIND_SERVER",
				StartTime:          tstr,
				EndTime:            tstr,
				ResourceAttributes: tmap,
				TraceAttributes:    newMapFromAttr(`{"traceAttribKey":"traceAttribVal", "scope.name":"testscope", "scope.version":"1.0"}`),
				Events: []*Event{
					{
						EventName:       "eventName",
						EventAttributes: newMapFromAttr(`{"eventkey": "eventvalue"}`),
						Timestamp:       tstr,
					},
				},
				Links: []*Link{{
					TraceID:            "00000000000000000000000000000064",
					SpanID:             "0000000000000032",
					TraceState:         string(ptrace.TraceStateEmpty),
					SpanLinkAttributes: newMapFromAttr(`{}`),
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.expectedAdxTrace
			got := mapToAdxTrace(tt.resourceFn(), tt.insScopeFn(), tt.spanDatafn())
			require.NotNil(t, got)
			assert.Equal(t, want, got)

		})
	}

}

func getEmptyEvents() []*Event {
	return []*Event{}

}

func getEmptyLinks() []*Link {
	return []*Link{}
}

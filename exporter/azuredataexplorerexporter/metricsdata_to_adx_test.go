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
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func Test_mapToAdxMetric(t *testing.T) {
	tsUnix := time.Unix(time.Now().Unix(), time.Now().UnixNano())
	ts := pcommon.NewTimestampFromTime(tsUnix)
	tstr := ts.AsTime().Format(time.RFC3339)

	tests := []struct {
		name               string
		resourceFn         func() pcommon.Resource
		metricsDataFn      func() pmetric.Metric
		expectedAdxMetrics []*AdxMetric
		configFn           func() *Config
	}{
		{
			name: "simple_counter_with_double_value",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("counter_over_time")
				sumV.SetDataType(pmetric.MetricDataTypeSum)
				dp := sumV.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(22.0)
				dp.SetTimestamp(ts)
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},

			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:  tstr,
					MetricName: "counter_over_time",
					MetricType: "Sum",
					Value:      22.0,
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
			},
		},
		{
			name: "simple_counter_with_int_value",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("int_counter_over_time")
				sumV.SetDataType(pmetric.MetricDataTypeSum)
				dp := sumV.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(221)
				dp.SetTimestamp(ts)
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},

			expectedAdxMetrics: []*AdxMetric{
				{
					Timestamp:  tstr,
					MetricName: "int_counter_over_time",
					MetricType: "Sum",
					Value:      221,
					Host:       "test-host",
					Attributes: `{"key":"value"}`,
				},
			},
		},
		{
			name: "nil_counter",
			resourceFn: func() pcommon.Resource {
				return newMetricsWithResources()
			},
			metricsDataFn: func() pmetric.Metric {
				sumV := pmetric.NewMetric()
				sumV.SetName("nil_counter_over_time")
				sumV.SetDataType(pmetric.MetricDataTypeSum)
				return sumV
			},
			configFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.resourceFn()
			md := tt.metricsDataFn()
			actualMetrics := mapToAdxMetric(res, md, zap.NewNop())
			encoder := json.NewEncoder(ioutil.Discard)
			for i, expected := range tt.expectedAdxMetrics {
				assert.Equal(t, expected, actualMetrics[i])
				err := encoder.Encode(actualMetrics[i])
				assert.NoError(t, err)
			}
		})
	}
}

func newMetricsWithResources() pcommon.Resource {
	res := pcommon.NewResource()
	res.Attributes().InsertString("key", "value")
	res.Attributes().InsertString(hostKey, "test-host")
	return res
}

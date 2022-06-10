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
			name: "simple_counter",
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
					Attributes: `{"k0":"v0","k1":"v1"}`,
				},
			},
		},
		/*
			{
				name: "nil_gauge_value",
				resourceFn: func() pcommon.Resource {
					return newMetricsWithResources()
				},
				metricsDataFn: func() pmetric.Metric {
					gauge := pmetric.NewMetric()
					gauge.SetName("gauge_with_dims")
					gauge.SetDataType(pmetric.MetricDataTypeGauge)
					return gauge
				},
				configFn: func() *Config {
					return createDefaultConfig().(*Config)
				},
			},
		*/
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

func getAdxMetrics(
	metricName string,
	ts *float64,
	keys []string,
	values []interface{},
	val interface{},
	source string,
	host string,
) []*AdxMetric {

	fVal := float64(0)
	adxMetrics := make([]*AdxMetric, 1)

	adxMetrics[0] = &AdxMetric{
		Timestamp:  "",
		MetricName: "",
		MetricType: "",
		Value:      fVal,
		Host:       "",
		Attributes: "",
	}
	return adxMetrics

}

func newMetricsWithResources() pcommon.Resource {
	res := pcommon.NewResource()
	res.Attributes().InsertString("k0", "v0")
	res.Attributes().InsertString("k1", "v1")
	res.Attributes().InsertString(hostKey, "test-host")
	return res
}

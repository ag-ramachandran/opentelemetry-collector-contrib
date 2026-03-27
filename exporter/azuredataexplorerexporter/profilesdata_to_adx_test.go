// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func Test_mapToAdxProfiles(t *testing.T) {
	epoch, _ := time.Parse("2006-01-02T15:04:05.999999999Z07:00", "1970-01-01T00:00:00.000000000Z")
	defaultTime := pcommon.NewTimestampFromTime(epoch).AsTime().Format(time.RFC3339Nano)

	tests := []struct {
		name             string
		profilesFn       func() (pcommon.Resource, pcommon.InstrumentationScope, pprofile.ProfilesDictionary, pprofile.Profile)
		expectedProfiles []*adxProfile
	}{
		{
			name: "single_sample_with_stack",
			profilesFn: func() (pcommon.Resource, pcommon.InstrumentationScope, pprofile.ProfilesDictionary, pprofile.Profile) {
				profiles := pprofile.NewProfiles()
				dic := profiles.Dictionary()

				// string_table: [0]="" [1]="cpu" [2]="nanoseconds" [3]="main" [4]="app.go" [5]="runtime.start" [6]="runtime.go"
				dic.StringTable().Append("")
				dic.StringTable().Append("cpu")
				dic.StringTable().Append("nanoseconds")
				dic.StringTable().Append("main")
				dic.StringTable().Append("app.go")
				dic.StringTable().Append("runtime.start")
				dic.StringTable().Append("runtime.go")

				// function_table: [0]=zero, [1]=main, [2]=runtime.start
				dic.FunctionTable().AppendEmpty()
				fn1 := dic.FunctionTable().AppendEmpty()
				fn1.SetNameStrindex(3)
				fn1.SetFilenameStrindex(4)
				fn1.SetStartLine(10)
				fn2 := dic.FunctionTable().AppendEmpty()
				fn2.SetNameStrindex(5)
				fn2.SetFilenameStrindex(6)
				fn2.SetStartLine(1)

				// mapping_table: [0]=zero
				dic.MappingTable().AppendEmpty()

				// location_table: [0]=zero, [1]=main@42, [2]=runtime.start@5
				dic.LocationTable().AppendEmpty()
				loc1 := dic.LocationTable().AppendEmpty()
				loc1.SetAddress(0x1000)
				line1 := loc1.Lines().AppendEmpty()
				line1.SetFunctionIndex(1)
				line1.SetLine(42)

				loc2 := dic.LocationTable().AppendEmpty()
				loc2.SetAddress(0x2000)
				line2 := loc2.Lines().AppendEmpty()
				line2.SetFunctionIndex(2)
				line2.SetLine(5)

				// stack_table: [0]=zero, [1]=[loc1, loc2] (leaf-first)
				dic.StackTable().AppendEmpty()
				stack := dic.StackTable().AppendEmpty()
				stack.LocationIndices().Append(1) // main (leaf)
				stack.LocationIndices().Append(2) // runtime.start (caller)

				// link_table: [0]=zero
				dic.LinkTable().AppendEmpty()
				// attribute_table: [0]=zero
				dic.AttributeTable().AppendEmpty()

				resource := pcommon.NewResource()
				resource.Attributes().PutStr("service.name", "test-svc")

				scope := pcommon.NewInstrumentationScope()
				scope.SetName("testprofiler")
				scope.SetVersion("0.1")

				profile := pprofile.NewProfile()
				profile.SetTime(ts)
				profile.SetDurationNano(1000000000)
				profile.SampleType().SetTypeStrindex(1)
				profile.SampleType().SetUnitStrindex(2)

				sample := profile.Samples().AppendEmpty()
				sample.SetStackIndex(1)
				sample.Values().Append(500000000)

				return resource, scope, dic, profile
			},
			expectedProfiles: []*adxProfile{
				{
					Timestamp:   tstr,
					Duration:    "1s",
					ProfileID:   "00000000000000000000000000000000",
					SampleType:  "cpu",
					SampleUnit:  "nanoseconds",
					SampleValue: 500000000,
					StackTrace: []*adxProfileFrame{
						{Function: "main", File: "app.go", Line: 42, Address: 0x1000},
						{Function: "runtime.start", File: "runtime.go", Line: 5, Address: 0x2000},
					},
					StackDepth:         2,
					SampleAttributes:   map[string]any{},
					ResourceAttributes: map[string]any{"service.name": "test-svc"},
					ScopeAttributes:    map[string]any{"scope.name": "testprofiler", "scope.version": "0.1"},
				},
			},
		},
		{
			name: "empty_profile",
			profilesFn: func() (pcommon.Resource, pcommon.InstrumentationScope, pprofile.ProfilesDictionary, pprofile.Profile) {
				profiles := pprofile.NewProfiles()
				dic := profiles.Dictionary()
				dic.StringTable().Append("")
				dic.FunctionTable().AppendEmpty()
				dic.MappingTable().AppendEmpty()
				dic.LocationTable().AppendEmpty()
				dic.StackTable().AppendEmpty()
				dic.LinkTable().AppendEmpty()
				dic.AttributeTable().AppendEmpty()

				resource := pcommon.NewResource()
				scope := pcommon.NewInstrumentationScope()
				profile := pprofile.NewProfile()

				return resource, scope, dic, profile
			},
			expectedProfiles: nil,
		},
		{
			name: "timestamp_only_samples",
			profilesFn: func() (pcommon.Resource, pcommon.InstrumentationScope, pprofile.ProfilesDictionary, pprofile.Profile) {
				profiles := pprofile.NewProfiles()
				dic := profiles.Dictionary()

				dic.StringTable().Append("")
				dic.StringTable().Append("cpu")
				dic.StringTable().Append("nanoseconds")
				dic.StringTable().Append("doWork")
				dic.StringTable().Append("work.go")

				dic.FunctionTable().AppendEmpty()
				fn := dic.FunctionTable().AppendEmpty()
				fn.SetNameStrindex(3)
				fn.SetFilenameStrindex(4)

				dic.MappingTable().AppendEmpty()
				dic.LocationTable().AppendEmpty()
				loc := dic.LocationTable().AppendEmpty()
				line := loc.Lines().AppendEmpty()
				line.SetFunctionIndex(1)
				line.SetLine(100)

				dic.StackTable().AppendEmpty()
				stack := dic.StackTable().AppendEmpty()
				stack.LocationIndices().Append(1)

				dic.LinkTable().AppendEmpty()
				dic.AttributeTable().AppendEmpty()

				resource := pcommon.NewResource()
				scope := pcommon.NewInstrumentationScope()

				profile := pprofile.NewProfile()
				profile.SampleType().SetTypeStrindex(1)
				profile.SampleType().SetUnitStrindex(2)

				sample := profile.Samples().AppendEmpty()
				sample.SetStackIndex(1)
				// No values, only timestamps → value should be 1
				sample.TimestampsUnixNano().Append(1000000000)
				sample.TimestampsUnixNano().Append(2000000000)

				return resource, scope, dic, profile
			},
			expectedProfiles: []*adxProfile{
				{
					Timestamp:   "1970-01-01T00:00:01Z",
					Duration:    "0s",
					ProfileID:   "00000000000000000000000000000000",
					SampleType:  "cpu",
					SampleUnit:  "nanoseconds",
					SampleValue: 1,
					StackTrace: []*adxProfileFrame{
						{Function: "doWork", File: "work.go", Line: 100},
					},
					StackDepth:         1,
					SampleAttributes:   map[string]any{},
					ResourceAttributes: map[string]any{},
					ScopeAttributes:    map[string]any{},
				},
				{
					Timestamp:   "1970-01-01T00:00:02Z",
					Duration:    "0s",
					ProfileID:   "00000000000000000000000000000000",
					SampleType:  "cpu",
					SampleUnit:  "nanoseconds",
					SampleValue: 1,
					StackTrace: []*adxProfileFrame{
						{Function: "doWork", File: "work.go", Line: 100},
					},
					StackDepth:         1,
					SampleAttributes:   map[string]any{},
					ResourceAttributes: map[string]any{},
					ScopeAttributes:    map[string]any{},
				},
			},
		},
		{
			name: "sample_with_link",
			profilesFn: func() (pcommon.Resource, pcommon.InstrumentationScope, pprofile.ProfilesDictionary, pprofile.Profile) {
				profiles := pprofile.NewProfiles()
				dic := profiles.Dictionary()

				dic.StringTable().Append("")
				dic.StringTable().Append("cpu")
				dic.StringTable().Append("nanoseconds")
				dic.StringTable().Append("handler")
				dic.StringTable().Append("server.go")

				dic.FunctionTable().AppendEmpty()
				fn := dic.FunctionTable().AppendEmpty()
				fn.SetNameStrindex(3)
				fn.SetFilenameStrindex(4)

				dic.MappingTable().AppendEmpty()
				dic.LocationTable().AppendEmpty()
				loc := dic.LocationTable().AppendEmpty()
				line := loc.Lines().AppendEmpty()
				line.SetFunctionIndex(1)
				line.SetLine(25)

				dic.StackTable().AppendEmpty()
				stack := dic.StackTable().AppendEmpty()
				stack.LocationIndices().Append(1)

				// link_table: [0]=zero, [1]=Link with traceID/spanID
				dic.LinkTable().AppendEmpty()
				lnk := dic.LinkTable().AppendEmpty()
				lnk.SetTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100})
				lnk.SetSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 50})

				dic.AttributeTable().AppendEmpty()

				resource := pcommon.NewResource()
				scope := pcommon.NewInstrumentationScope()

				profile := pprofile.NewProfile()
				profile.SetTime(ts)
				profile.SampleType().SetTypeStrindex(1)
				profile.SampleType().SetUnitStrindex(2)

				sample := profile.Samples().AppendEmpty()
				sample.SetStackIndex(1)
				sample.SetLinkIndex(1)
				sample.Values().Append(100)

				return resource, scope, dic, profile
			},
			expectedProfiles: []*adxProfile{
				{
					Timestamp:   tstr,
					Duration:    "0s",
					ProfileID:   "00000000000000000000000000000000",
					SampleType:  "cpu",
					SampleUnit:  "nanoseconds",
					SampleValue: 100,
					StackTrace: []*adxProfileFrame{
						{Function: "handler", File: "server.go", Line: 25},
					},
					StackDepth:         1,
					TraceID:            "00000000000000000000000000000064",
					SpanID:             "0000000000000032",
					SampleAttributes:   map[string]any{},
					ResourceAttributes: map[string]any{},
					ScopeAttributes:    map[string]any{},
				},
			},
		},
		{
			name: "inlined_functions",
			profilesFn: func() (pcommon.Resource, pcommon.InstrumentationScope, pprofile.ProfilesDictionary, pprofile.Profile) {
				profiles := pprofile.NewProfiles()
				dic := profiles.Dictionary()

				// string_table: [0]="" [1]="cpu" [2]="ns" [3]="memcpy" [4]="string.c" [5]="printf" [6]="stdio.c"
				dic.StringTable().Append("")
				dic.StringTable().Append("cpu")
				dic.StringTable().Append("ns")
				dic.StringTable().Append("memcpy")
				dic.StringTable().Append("string.c")
				dic.StringTable().Append("printf")
				dic.StringTable().Append("stdio.c")

				// function_table: [0]=zero, [1]=memcpy, [2]=printf
				dic.FunctionTable().AppendEmpty()
				fn1 := dic.FunctionTable().AppendEmpty()
				fn1.SetNameStrindex(3) // memcpy
				fn1.SetFilenameStrindex(4)
				fn2 := dic.FunctionTable().AppendEmpty()
				fn2.SetNameStrindex(5) // printf
				fn2.SetFilenameStrindex(6)

				dic.MappingTable().AppendEmpty()

				// location with two lines (inlined memcpy into printf)
				dic.LocationTable().AppendEmpty()
				loc := dic.LocationTable().AppendEmpty()
				loc.SetAddress(0x3000)
				l1 := loc.Lines().AppendEmpty()
				l1.SetFunctionIndex(1) // memcpy (inlined)
				l1.SetLine(10)
				l2 := loc.Lines().AppendEmpty()
				l2.SetFunctionIndex(2) // printf (caller)
				l2.SetLine(200)

				dic.StackTable().AppendEmpty()
				stack := dic.StackTable().AppendEmpty()
				stack.LocationIndices().Append(1)

				dic.LinkTable().AppendEmpty()
				dic.AttributeTable().AppendEmpty()

				resource := pcommon.NewResource()
				scope := pcommon.NewInstrumentationScope()

				profile := pprofile.NewProfile()
				profile.SampleType().SetTypeStrindex(1)
				profile.SampleType().SetUnitStrindex(2)

				sample := profile.Samples().AppendEmpty()
				sample.SetStackIndex(1)
				sample.Values().Append(42)

				return resource, scope, dic, profile
			},
			expectedProfiles: []*adxProfile{
				{
					Timestamp:   defaultTime,
					Duration:    "0s",
					ProfileID:   "00000000000000000000000000000000",
					SampleType:  "cpu",
					SampleUnit:  "ns",
					SampleValue: 42,
					StackTrace: []*adxProfileFrame{
						{Function: "memcpy", File: "string.c", Line: 10, Address: 0x3000},
						{Function: "printf", File: "stdio.c", Line: 200, Address: 0x3000},
					},
					StackDepth:         2,
					SampleAttributes:   map[string]any{},
					ResourceAttributes: map[string]any{},
					ScopeAttributes:    map[string]any{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, scope, dic, profile := tt.profilesFn()
			got := mapToAdxProfiles(resource, scope, dic, profile)
			if tt.expectedProfiles == nil {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, len(tt.expectedProfiles))
			for i, want := range tt.expectedProfiles {
				assert.Equal(t, want, got[i])
				// Verify it serializes cleanly to JSON
				encoder := json.NewEncoder(io.Discard)
				err := encoder.Encode(got[i])
				assert.NoError(t, err)
			}
		})
	}
}

func Test_rawProfilesToAdxProfiles(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()

	dic.StringTable().Append("")
	dic.StringTable().Append("cpu")
	dic.StringTable().Append("nanoseconds")
	dic.StringTable().Append("main")
	dic.StringTable().Append("app.go")

	dic.FunctionTable().AppendEmpty()
	fn := dic.FunctionTable().AppendEmpty()
	fn.SetNameStrindex(3)
	fn.SetFilenameStrindex(4)

	dic.MappingTable().AppendEmpty()
	dic.LocationTable().AppendEmpty()
	loc := dic.LocationTable().AppendEmpty()
	line := loc.Lines().AppendEmpty()
	line.SetFunctionIndex(1)
	line.SetLine(42)

	dic.StackTable().AppendEmpty()
	stack := dic.StackTable().AppendEmpty()
	stack.LocationIndices().Append(1)

	dic.LinkTable().AppendEmpty()
	dic.AttributeTable().AppendEmpty()

	rp := profiles.ResourceProfiles().AppendEmpty()
	rp.Resource().Attributes().PutStr("service.name", "test-svc")
	sp := rp.ScopeProfiles().AppendEmpty()
	sp.Scope().SetName("profiler")
	sp.Scope().SetVersion("1.0")

	profile := sp.Profiles().AppendEmpty()
	profile.SetTime(ts)
	profile.SampleType().SetTypeStrindex(1)
	profile.SampleType().SetUnitStrindex(2)

	sample := profile.Samples().AppendEmpty()
	sample.SetStackIndex(1)
	sample.Values().Append(99)

	results := rawProfilesToAdxProfiles(profiles)
	require.Len(t, results, 1)
	assert.Equal(t, "cpu", results[0].SampleType)
	assert.Equal(t, "nanoseconds", results[0].SampleUnit)
	assert.Equal(t, int64(99), results[0].SampleValue)
	assert.Len(t, results[0].StackTrace, 1)
	assert.Equal(t, "main", results[0].StackTrace[0].Function)
	assert.Equal(t, "app.go", results[0].StackTrace[0].File)
	assert.Equal(t, int64(42), results[0].StackTrace[0].Line)
	assert.Equal(t, map[string]any{"service.name": "test-svc"}, results[0].ResourceAttributes)
	assert.Equal(t, map[string]any{"scope.name": "profiler", "scope.version": "1.0"}, results[0].ScopeAttributes)
}

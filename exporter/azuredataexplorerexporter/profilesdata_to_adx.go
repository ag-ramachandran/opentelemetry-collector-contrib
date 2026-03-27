// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"encoding/hex"
	"maps"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

// adxProfileFrame represents a single resolved stack frame in a profile sample.
type adxProfileFrame struct {
	Function   string `json:"Function"`
	SystemName string `json:"SystemName,omitempty"`
	File       string `json:"File,omitempty"`
	Line       int64  `json:"Line,omitempty"`
	Column     int64  `json:"Column,omitempty"`
	Mapping    string `json:"Mapping,omitempty"`
	Address    uint64 `json:"Address,omitempty"`
}

// adxProfile represents a flattened profile sample row for ADX ingestion.
// Each sample becomes one row with a fully resolved stack trace.
type adxProfile struct {
	Timestamp          string             `json:"Timestamp"`
	Duration           string             `json:"Duration"`
	ProfileID          string             `json:"ProfileID"`
	SampleType         string             `json:"SampleType"`
	SampleUnit         string             `json:"SampleUnit"`
	SampleValue        int64              `json:"SampleValue"`
	StackTrace         []*adxProfileFrame `json:"StackTrace"`
	StackDepth         int                `json:"StackDepth"`
	TraceID            string             `json:"TraceID,omitempty"`
	SpanID             string             `json:"SpanID,omitempty"`
	SampleAttributes   map[string]any     `json:"SampleAttributes"`
	ResourceAttributes map[string]any     `json:"ResourceAttributes"`
	ScopeAttributes    map[string]any     `json:"ScopeAttributes"`
}

// resolveString looks up a string from the ProfilesDictionary string table.
func resolveString(dic pprofile.ProfilesDictionary, index int32) string {
	st := dic.StringTable()
	if index <= 0 || int(index) >= st.Len() {
		return ""
	}
	return st.At(int(index))
}

// resolveStackTrace resolves a stack index into a slice of adxProfileFrame
// by walking Stack → Location → Line → Function through the dictionary.
func resolveStackTrace(dic pprofile.ProfilesDictionary, stackIndex int32) []*adxProfileFrame {
	stackTable := dic.StackTable()
	if stackIndex <= 0 || int(stackIndex) >= stackTable.Len() {
		return []*adxProfileFrame{}
	}

	stack := stackTable.At(int(stackIndex))
	locIndices := stack.LocationIndices()
	var frames []*adxProfileFrame

	locationTable := dic.LocationTable()
	functionTable := dic.FunctionTable()
	mappingTable := dic.MappingTable()

	for li := 0; li < locIndices.Len(); li++ {
		locIdx := locIndices.At(li)
		if int(locIdx) <= 0 || int(locIdx) >= locationTable.Len() {
			continue
		}
		loc := locationTable.At(int(locIdx))

		// Resolve the mapping filename for this location
		var mappingFilename string
		mapIdx := loc.MappingIndex()
		if mapIdx > 0 && int(mapIdx) < mappingTable.Len() {
			m := mappingTable.At(int(mapIdx))
			mappingFilename = resolveString(dic, m.FilenameStrindex())
		}

		lines := loc.Lines()
		if lines.Len() == 0 {
			// Location with no lines — emit a frame with just the address
			frames = append(frames, &adxProfileFrame{
				Address: loc.Address(),
				Mapping: mappingFilename,
			})
			continue
		}

		for lni := 0; lni < lines.Len(); lni++ {
			line := lines.At(lni)
			fnIdx := line.FunctionIndex()
			var fnName, sysName, fileName string
			if fnIdx > 0 && int(fnIdx) < functionTable.Len() {
				fn := functionTable.At(int(fnIdx))
				fnName = resolveString(dic, fn.NameStrindex())
				sysName = resolveString(dic, fn.SystemNameStrindex())
				fileName = resolveString(dic, fn.FilenameStrindex())
			}
			frames = append(frames, &adxProfileFrame{
				Function:   fnName,
				SystemName: sysName,
				File:       fileName,
				Line:       line.Line(),
				Column:     line.Column(),
				Mapping:    mappingFilename,
				Address:    loc.Address(),
			})
		}
	}

	return frames
}

// resolveAttributes resolves attribute indices from the dictionary attribute table
// into a standard map.
func resolveAttributes(dic pprofile.ProfilesDictionary, indices pcommon.Int32Slice) map[string]any {
	attrs := make(map[string]any)
	attrTable := dic.AttributeTable()
	for i := 0; i < indices.Len(); i++ {
		idx := indices.At(i)
		if idx <= 0 || int(idx) >= attrTable.Len() {
			continue
		}
		kv := attrTable.At(int(idx))
		key := resolveString(dic, kv.KeyStrindex())
		if key == "" {
			continue
		}
		attrs[key] = kv.Value().AsRaw()
	}
	return attrs
}

// mapToAdxProfiles converts a single Profile (with its samples) into a slice of adxProfile rows.
// Each sample becomes one row with the stack trace fully resolved from the dictionary.
func mapToAdxProfiles(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	dic pprofile.ProfilesDictionary,
	profile pprofile.Profile,
) []*adxProfile {
	timestamp := profile.Time().AsTime().Format(time.RFC3339Nano)
	durationNano := profile.DurationNano()
	duration := time.Duration(durationNano).String()

	pid := profile.ProfileID()
	profileID := hex.EncodeToString(pid[:])

	sampleType := resolveString(dic, profile.SampleType().TypeStrindex())
	sampleUnit := resolveString(dic, profile.SampleType().UnitStrindex())

	resourceAttrs := resource.Attributes().AsRaw()
	scopeAttrs := maps.Clone(getScopeMap(scope))

	linkTable := dic.LinkTable()

	samples := profile.Samples()
	var results []*adxProfile

	for si := 0; si < samples.Len(); si++ {
		sample := samples.At(si)

		stackTrace := resolveStackTrace(dic, sample.StackIndex())
		sampleAttrs := resolveAttributes(dic, sample.AttributeIndices())

		// Resolve link for trace correlation
		var traceID, spanID string
		linkIdx := sample.LinkIndex()
		if linkIdx > 0 && int(linkIdx) < linkTable.Len() {
			lnk := linkTable.At(int(linkIdx))
			tid := lnk.TraceID()
			sid := lnk.SpanID()
			traceID = hex.EncodeToString(tid[:])
			spanID = hex.EncodeToString(sid[:])
		}

		// A sample may have multiple values or timestamps.
		// For aggregated samples (values present), emit one row per value.
		// For timestamp-only samples, emit one row per timestamp with value=1.
		values := sample.Values()
		timestamps := sample.TimestampsUnixNano()

		if values.Len() > 0 {
			for vi := 0; vi < values.Len(); vi++ {
				ts := timestamp
				if vi < timestamps.Len() {
					ts = time.Unix(0, int64(timestamps.At(vi))).UTC().Format(time.RFC3339Nano)
				}
				results = append(results, &adxProfile{
					Timestamp:          ts,
					Duration:           duration,
					ProfileID:          profileID,
					SampleType:         sampleType,
					SampleUnit:         sampleUnit,
					SampleValue:        values.At(vi),
					StackTrace:         stackTrace,
					StackDepth:         len(stackTrace),
					TraceID:            traceID,
					SpanID:             spanID,
					SampleAttributes:   sampleAttrs,
					ResourceAttributes: resourceAttrs,
					ScopeAttributes:    scopeAttrs,
				})
			}
		} else if timestamps.Len() > 0 {
			// Timestamp-only samples: value is implicitly 1
			for ti := 0; ti < timestamps.Len(); ti++ {
				ts := time.Unix(0, int64(timestamps.At(ti))).UTC().Format(time.RFC3339Nano)
				results = append(results, &adxProfile{
					Timestamp:          ts,
					Duration:           duration,
					ProfileID:          profileID,
					SampleType:         sampleType,
					SampleUnit:         sampleUnit,
					SampleValue:        1,
					StackTrace:         stackTrace,
					StackDepth:         len(stackTrace),
					TraceID:            traceID,
					SpanID:             spanID,
					SampleAttributes:   sampleAttrs,
					ResourceAttributes: resourceAttrs,
					ScopeAttributes:    scopeAttrs,
				})
			}
		}
	}
	return results
}

// rawProfilesToAdxProfiles converts full OTLP Profiles data to a flat slice of adxProfile rows.
func rawProfilesToAdxProfiles(profiles pprofile.Profiles) []*adxProfile {
	dic := profiles.Dictionary()
	var results []*adxProfile
	resourceProfiles := profiles.ResourceProfiles()
	for i := 0; i < resourceProfiles.Len(); i++ {
		rp := resourceProfiles.At(i)
		res := rp.Resource()
		scopeProfiles := rp.ScopeProfiles()
		for j := 0; j < scopeProfiles.Len(); j++ {
			sp := scopeProfiles.At(j)
			scope := sp.Scope()
			profs := sp.Profiles()
			for k := 0; k < profs.Len(); k++ {
				results = append(results, mapToAdxProfiles(res, scope, dic, profs.At(k))...)
			}
		}
	}
	return results
}

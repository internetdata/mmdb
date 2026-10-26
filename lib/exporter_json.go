// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/oschwald/maxminddb-golang/v2"
)

// jsonExporter exports records in JSON Lines format.
type jsonExporter struct {
	bw    *bufio.Writer
	cache map[uintptr]jsonRecord
}

// jsonRecord is a record's JSON with an empty "range" where it sorts among the
// keys, so each network's range is written at cut and no data is ever searched.
type jsonRecord struct {
	line []byte
	cut  int
}

func newJSONExporter(w io.Writer) *jsonExporter {
	return &jsonExporter{
		bw:    bufio.NewWriter(w),
		cache: make(map[uintptr]jsonRecord),
	}
}

func (e *jsonExporter) WriteRecord(result maxminddb.Result) error {
	offset := result.Offset()

	cached, ok := e.cache[offset]
	if !ok {
		record := make(map[string]any)
		if err := result.Decode(&record); err != nil {
			return fmt.Errorf("failed to decode record: %w", err)
		}
		encoded, err := encodeJSONRecord(record)
		if err != nil {
			return fmt.Errorf("failed to encode record: %w", err)
		}
		cached = encoded
		e.cache[offset] = cached
	}

	e.bw.Write(cached.line[:cached.cut])
	e.bw.WriteString(result.Prefix().String())
	e.bw.Write(cached.line[cached.cut:])
	e.bw.WriteByte('\n')
	return nil
}

func (e *jsonExporter) Flush() error {
	return e.bw.Flush()
}

// encodeJSONRecord drops a "range" key of the record's own: the network's
// range replaces it.
func encodeJSONRecord(record map[string]any) (jsonRecord, error) {
	delete(record, "range")
	before := make(map[string]any)
	for k, v := range record {
		if k < "range" {
			before[k] = v
			delete(record, k)
		}
	}
	headObj, err := json.Marshal(before)
	if err != nil {
		return jsonRecord{}, err
	}
	tailObj, err := json.Marshal(record)
	if err != nil {
		return jsonRecord{}, err
	}

	line := make([]byte, 0, len(headObj)+len(`,"range":"",`)+len(tailObj))
	line = append(line, headObj[:len(headObj)-1]...)
	if len(before) > 0 {
		line = append(line, ',')
	}
	line = append(line, `"range":"`...)
	cut := len(line)
	line = append(line, '"')
	if len(record) > 0 {
		line = append(line, ',')
	}
	line = append(line, tailObj[1:]...)
	return jsonRecord{line: line, cut: cut}, nil
}

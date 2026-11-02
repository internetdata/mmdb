// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/edsrzf/mmap-go"
)

func sortedMapKeys(m map[string]string) []string {
	i := 0
	keys := make([]string, len(m))
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeysAndVals(m map[string]string) ([]string, []string) {
	keys := sortedMapKeys(m)
	vals := make([]string, len(keys))
	for i, k := range keys {
		vals[i] = m[k]
	}
	return keys, vals
}

func longestStrInStringSlice(s []string) string {
	_longest := ""
	longest := &_longest
	for i := 0; i < len(s); i++ {
		if len(s[i]) > len(*longest) {
			longest = &s[i]
		}
	}
	return *longest
}

func mapInterfaceToStr(m map[string]interface{}) map[string]string {
	retVal := make(map[string]string)
	for key, value := range m {
		switch v := value.(type) {
		case int:
			retVal[key] = strconv.Itoa(v)
		case float64:
			retVal[key] = fmt.Sprintf("%f", v)
		case string:
			retVal[key] = v
		default:
			outJson, err := json.Marshal(v)
			if err != nil {
				return nil
			}
			retVal[key] = string(outJson)
		}
	}
	return retVal
}

func findSectionSeparator(mmdbFile string, sep string) (int64, error) {
	file, err := os.Open(mmdbFile)
	if err != nil {
		return 0, fmt.Errorf("couldn't open mmdb file: %w", err)
	}
	defer file.Close()

	// Map the mmdb file into memory.
	mmap, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer mmap.Unmap()

	// Search the last occurrence of the separator in the file.
	index := bytes.LastIndex(mmap, []byte(sep))
	if index != -1 {
		return int64(index), nil
	}

	return -1, nil
}

func simplifySize(size int64) string {
	const (
		_  = iota
		KB = 1 << (10 * iota)
		MB
		GB
		TB
	)

	if size >= TB {
		return fmt.Sprintf("(%.2f TB)", float64(size)/float64(TB))
	} else if size >= GB {
		return fmt.Sprintf("(%.2f GB)", float64(size)/float64(GB))
	} else if size >= MB {
		return fmt.Sprintf("(%.2f MB)", float64(size)/float64(MB))
	} else if size >= KB {
		return fmt.Sprintf("(%.2f KB)", float64(size)/float64(KB))
	} else {
		return ""
	}
}

func simplifyCount(size int64) string {
	const (
		K = 1e3
		M = 1e6
		G = 1e9
		T = 1e12
	)

	if size >= T {
		return fmt.Sprintf("(%.2f T)", float64(size)/float64(T))
	} else if size >= G {
		return fmt.Sprintf("(%.2f G)", float64(size)/float64(G))
	} else if size >= M {
		return fmt.Sprintf("(%.2f M)", float64(size)/float64(M))
	} else if size >= K {
		return fmt.Sprintf("(%.2f K)", float64(size)/float64(K))
	} else {
		return ""
	}
}

type TypeSizes struct {
	PointerSize           int64 `json:"pointer_size"`
	Utf8StringSize        int64 `json:"utf8_string_size"`
	DoubleSize            int64 `json:"double_size"`
	BytesSize             int64 `json:"bytes_size"`
	Unsigned16bitIntSize  int64 `json:"unsigned_16bit_int_size"`
	Unsigned32bitIntSize  int64 `json:"unsigned_32bit_int_size"`
	Signed32bitIntSize    int64 `json:"signed_32bit_int_size"`
	Unsigned64bitIntSize  int64 `json:"unsigned_64bit_int_size"`
	Unsigned128bitIntSize int64 `json:"unsigned_128bit_int_size"`
	MapKeyValueCount      int64 `json:"map_key_value_pair_count"`
	ArrayLength           int64 `json:"array_length"`
	FloatSize             int64 `json:"float_size"`
}

func traverseDataSection(mmdbFile string, startOffset int64, endOffset int64) (TypeSizes, error) {
	file, err := os.Open(mmdbFile)
	if err != nil {
		return TypeSizes{}, fmt.Errorf("couldn't open mmdb file: %w", err)
	}
	defer file.Close()

	mmap, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		return TypeSizes{}, err
	}
	defer mmap.Unmap()

	if startOffset < 0 || startOffset > endOffset || endOffset > int64(len(mmap)) {
		return TypeSizes{}, fmt.Errorf("the data section, %d to %d, is not inside the file", startOffset, endOffset)
	}
	return dataTypeSizes(mmap[startOffset:endOffset])
}

// dataTypeSizes walks a data section value by value, so a value a pointer
// points at is counted once, where it is stored.
func dataTypeSizes(data []byte) (TypeSizes, error) {
	var sizes TypeSizes
	for i := 0; i < len(data); {
		start := i
		ctrl := data[i]
		i++
		dataType := int(ctrl >> 5)
		if dataType == 1 {
			n := int(ctrl>>3&0b11) + 1
			if len(data)-i < n {
				return TypeSizes{}, truncatedValueError(start)
			}
			sizes.PointerSize += int64(n)
			i += n
			continue
		}
		if dataType == 0 {
			if i == len(data) {
				return TypeSizes{}, truncatedValueError(start)
			}
			dataType = 7 + int(data[i])
			i++
			if dataType < 8 {
				return TypeSizes{}, fmt.Errorf("invalid extended type at data section offset %d", start)
			}
		}

		size := int(ctrl & 0b11111)
		if size >= 29 {
			n := size - 28
			if len(data)-i < n {
				return TypeSizes{}, truncatedValueError(start)
			}
			ext := 0
			for _, b := range data[i : i+n] {
				ext = ext<<8 | int(b)
			}
			size = [...]int{29, 285, 65821}[n-1] + ext
			i += n
		}

		var payload *int64
		switch dataType {
		case 2:
			payload = &sizes.Utf8StringSize
		case 3:
			payload = &sizes.DoubleSize
		case 4:
			payload = &sizes.BytesSize
		case 5:
			payload = &sizes.Unsigned16bitIntSize
		case 6:
			payload = &sizes.Unsigned32bitIntSize
		case 7:
			sizes.MapKeyValueCount += int64(size)
			continue
		case 8:
			payload = &sizes.Signed32bitIntSize
		case 9:
			payload = &sizes.Unsigned64bitIntSize
		case 10:
			payload = &sizes.Unsigned128bitIntSize
		case 11:
			sizes.ArrayLength += int64(size)
			continue
		case 12, 13, 14:
			// No payload: a cache container, an end marker, or a boolean, whose size is its value.
			continue
		case 15:
			payload = &sizes.FloatSize
		default:
			return TypeSizes{}, fmt.Errorf("unknown data type %d at data section offset %d", dataType, start)
		}
		if len(data)-i < size {
			return TypeSizes{}, truncatedValueError(start)
		}
		*payload += int64(size)
		i += size
	}

	return sizes, nil
}

func truncatedValueError(offset int) error {
	return fmt.Errorf("the value at data section offset %d runs past the end of the section", offset)
}

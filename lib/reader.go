// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

type reader interface {
	Read() (record []string, err error)
}

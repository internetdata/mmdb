// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package lib

type writer interface {
	Write(record []string) error
	Flush()
	Error() error
}

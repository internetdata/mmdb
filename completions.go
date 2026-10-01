// Modifications copyright (C) 2025 InternetData
// This file has been modified from its original Apache-licensed version.
package main

import (
	complete "github.com/mslmio/libgo-complete"
	"github.com/mslmio/libgo-complete/predict"
)

var completions = &complete.Command{
	Sub: map[string]*complete.Command{
		"read":       completionsRead,
		"import":     completionsImport,
		"export":     completionsExport,
		"diff":       completionsDiff,
		"metadata":   completionsMetadata,
		"verify":     completionsVerify,
		"compress":   completionsCompress,
		"completion": completionsCompletion,
		"version":    {},
	},
	Flags: map[string]complete.Predictor{
		"--nocolor": predict.Nothing,
		"-h":        predict.Nothing,
		"--help":    predict.Nothing,
	},
}

func handleCompletions() {
	completions.Complete(progBase)
}

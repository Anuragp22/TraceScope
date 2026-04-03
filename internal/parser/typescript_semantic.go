package parser

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

type tsSemanticRequest struct {
	Files []string `json:"files"`
}

type tsSemanticFile struct {
	FilePath string           `json:"filePath"`
	Calls    []tsSemanticCall `json:"calls"`
}

type tsSemanticCall struct {
	Line            int    `json:"line"`
	Name            string `json:"name"`
	Receiver        string `json:"receiver,omitempty"`
	ReceiverType    string `json:"receiverType,omitempty"`
	ReceiverPackage string `json:"receiverPackage,omitempty"`
}

func enrichTypeScriptSemantics(results []*FileResult) {
	files := make([]string, 0)
	tsResults := make(map[string]*FileResult)
	for _, result := range results {
		if result == nil || result.Language != LangTypeScript {
			continue
		}
		files = append(files, result.FilePath)
		tsResults[canonicalSemanticPath(result.FilePath)] = result
	}
	if len(files) == 0 {
		return
	}

	scriptPath, ok := typeScriptSemanticScriptPath()
	if !ok {
		return
	}

	payload, err := json.Marshal(tsSemanticRequest{Files: files})
	if err != nil {
		return
	}

	cmd := exec.Command("node", scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	var semanticFiles []tsSemanticFile
	if err := json.Unmarshal(out, &semanticFiles); err != nil {
		return
	}

	for _, semanticFile := range semanticFiles {
		result := tsResults[canonicalSemanticPath(semanticFile.FilePath)]
		if result == nil {
			continue
		}
		mergeTypeScriptSemanticCalls(result, semanticFile.Calls)
	}
}

func mergeTypeScriptSemanticCalls(result *FileResult, semanticCalls []tsSemanticCall) {
	callIndex := make(map[string][]int)
	lineNameIndex := make(map[string][]int)
	for i, call := range result.Calls {
		callIndex[semanticCallKey(call.Line, call.Name, call.Receiver)] = append(callIndex[semanticCallKey(call.Line, call.Name, call.Receiver)], i)
		lineNameIndex[semanticCallKey(call.Line, call.Name, "")] = append(lineNameIndex[semanticCallKey(call.Line, call.Name, "")], i)
	}

	for _, semanticCall := range semanticCalls {
		if semanticCall.ReceiverType == "" {
			continue
		}
		idx := uniqueSemanticIndex(callIndex[semanticCallKey(semanticCall.Line, semanticCall.Name, semanticCall.Receiver)])
		if idx < 0 {
			idx = uniqueSemanticIndex(lineNameIndex[semanticCallKey(semanticCall.Line, semanticCall.Name, "")])
		}
		if idx < 0 {
			continue
		}
		result.Calls[idx].ReceiverType = semanticCall.ReceiverType
		result.Calls[idx].ReceiverPackage = semanticCall.ReceiverPackage
		if semanticCall.Receiver != "" {
			result.Calls[idx].Receiver = semanticCall.Receiver
		}
	}
}

// HasTypeScriptSemanticSupport reports whether compiler-backed TS enrichment
// can run in the current environment.
func HasTypeScriptSemanticSupport() bool {
	_, ok := typeScriptSemanticScriptPath()
	return ok
}

func typeScriptSemanticScriptPath() (string, bool) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	scriptPath := filepath.Join(filepath.Dir(currentFile), "typescript_semantic.mjs")
	if _, err := exec.LookPath("node"); err != nil {
		return "", false
	}
	if _, err := os.Stat(typeScriptRuntimePath(filepath.Dir(currentFile))); err != nil {
		return "", false
	}
	return scriptPath, true
}

func typeScriptRuntimePath(parserDir string) string {
	return filepath.Join(parserDir, "..", "..", "web", "node_modules", "typescript", "lib", "typescript.js")
}

func canonicalSemanticPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func semanticCallKey(line int, name, receiver string) string {
	return strconv.Itoa(line) + "|" + name + "|" + receiver
}

func uniqueSemanticIndex(indices []int) int {
	if len(indices) != 1 {
		return -1
	}
	return indices[0]
}

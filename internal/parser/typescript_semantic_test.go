package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_ParseFiles_EnrichesTypeScriptFactoryReturnTypes(t *testing.T) {
	if !HasTypeScriptSemanticSupport() {
		t.Skip("TypeScript compiler runtime not available")
	}

	dir := t.TempDir()

	servicePath := filepath.Join(dir, "service.ts")
	appPath := filepath.Join(dir, "app.ts")

	if err := os.WriteFile(servicePath, []byte(`
export class Service {
  run() {
    return "ok";
  }
}

export function createService() {
  return new Service();
}
`), 0o600); err != nil {
		t.Fatalf("write service.ts: %v", err)
	}

	if err := os.WriteFile(appPath, []byte(`
import { createService } from "./service";

export function main() {
  const service = createService();
  service.run();
}
`), 0o600); err != nil {
		t.Fatalf("write app.ts: %v", err)
	}

	results, errs := NewRegistry().ParseFiles(map[Language][]string{
		LangTypeScript: {servicePath, appPath},
	})
	if len(errs) > 0 {
		t.Fatalf("ParseFiles returned errors: %v", errs)
	}

	var appResult *FileResult
	for _, result := range results {
		if canonicalSemanticPath(result.FilePath) == canonicalSemanticPath(appPath) {
			appResult = result
			break
		}
	}
	if appResult == nil {
		t.Fatal("app.ts parse result not found")
	}

	foundTypedCall := false
	for _, call := range appResult.Calls {
		if call.Name == "run" && call.Receiver == "service" {
			if call.ReceiverType != "Service" {
				t.Fatalf("expected compiler-backed receiver type Service, got %q", call.ReceiverType)
			}
			if canonicalSemanticPath(call.ReceiverPackage) != canonicalSemanticPath(servicePath) {
				t.Fatalf("expected receiver package %q, got %q", servicePath, call.ReceiverPackage)
			}
			foundTypedCall = true
		}
	}
	if !foundTypedCall {
		t.Fatalf("expected service.run() call in %+v", appResult.Calls)
	}
}

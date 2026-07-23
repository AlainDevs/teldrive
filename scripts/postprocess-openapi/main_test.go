package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInsertOperationField_AddsAndIsIdempotent(t *testing.T) {
	const src = `openapi: 3.0.0
paths:
  /events/stream:
    get:
      responses:
        "200":
          description: ok
          content:
            text/event-stream:
              schema:
                type: string
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	patch := operationPatch{
		path:      "/events/stream",
		method:    "get",
		mediaType: "text/event-stream",
		fieldKey:  "x-ogen-raw-response",
		fieldBool: true,
	}

	if err := insertOperationField(&doc, patch); err != nil {
		t.Fatalf("insert patch: %v", err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if count := strings.Count(string(out), "x-ogen-raw-response: true"); count != 1 {
		t.Fatalf("expected exactly one raw-response extension after first patch, got %d\n%s", count, string(out))
	}

	if err := insertOperationField(&doc, patch); err != nil {
		t.Fatalf("insert patch second time: %v", err)
	}
	out2, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if count := strings.Count(string(out2), "x-ogen-raw-response: true"); count != 1 {
		t.Fatalf("expected exactly one raw-response extension after second patch, got %d\n%s", count, string(out2))
	}
}

func TestInsertOperationField_MissingPath(t *testing.T) {
	const src = `openapi: 3.0.0
paths: {}
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	err := insertOperationField(&doc, operationPatch{
		path:      "/events/stream",
		method:    "get",
		mediaType: "text/event-stream",
		fieldKey:  "x-ogen-raw-response",
		fieldBool: true,
	})
	if err == nil || !strings.Contains(err.Error(), "path block not found") {
		t.Fatalf("expected missing path error, got: %v", err)
	}
}

func TestInsertOperationField_MissingMethod(t *testing.T) {
	const src = `openapi: 3.0.0
paths:
  /events/stream:
    post:
      responses:
        "200":
          description: ok
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	err := insertOperationField(&doc, operationPatch{
		path:      "/events/stream",
		method:    "get",
		mediaType: "text/event-stream",
		fieldKey:  "x-ogen-raw-response",
		fieldBool: true,
	})
	if err == nil || !strings.Contains(err.Error(), "method block not found") {
		t.Fatalf("expected missing method error, got: %v", err)
	}
}

func TestInsertOperationField_MissingMediaType(t *testing.T) {
	const src = `openapi: 3.0.0
paths:
  /events/stream:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	err := insertOperationField(&doc, operationPatch{
		path:      "/events/stream",
		method:    "get",
		mediaType: "text/event-stream",
		fieldKey:  "x-ogen-raw-response",
		fieldBool: true,
	})
	if err == nil || !strings.Contains(err.Error(), "media type block not found") {
		t.Fatalf("expected missing media type error, got: %v", err)
	}
}

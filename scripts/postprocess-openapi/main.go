package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type operationPatch struct {
	path      string
	method    string
	mediaType string
	fieldKey  string
	fieldBool bool
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "postprocess-openapi: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	const filePath = "../openapi/openapi.yaml"

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return fmt.Errorf("unmarshal yaml: %w", err)
	}

	patches := []operationPatch{
		{
			path:      "/files/{id}/content",
			method:    "get",
			mediaType: "application/octet-stream",
			fieldKey:  "x-ogen-raw-response",
			fieldBool: true,
		},
		{
			path:      "/events/stream",
			method:    "get",
			mediaType: "text/event-stream",
			fieldKey:  "x-ogen-raw-response",
			fieldBool: true,
		},
		{
			path:      "/shares/{id}/files/{fileId}/content",
			method:    "get",
			mediaType: "application/octet-stream",
			fieldKey:  "x-ogen-raw-response",
			fieldBool: true,
		},
	}

	for _, patch := range patches {
		if err := insertOperationField(&doc, patch); err != nil {
			return err
		}
	}

	updated, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if string(updated) == string(content) {
		return nil
	}

	return os.WriteFile(filePath, updated, 0o644)
}

func insertOperationField(doc *yaml.Node, patch operationPatch) error {
	root, err := documentMapping(doc)
	if err != nil {
		return err
	}

	paths, ok := mappingValue(root, "paths")
	if !ok {
		return fmt.Errorf("paths block not found")
	}

	pathNode, ok := mappingValue(paths, patch.path)
	if !ok {
		return fmt.Errorf("path block not found: %s", patch.path)
	}

	methodNode, ok := mappingValue(pathNode, patch.method)
	if !ok {
		return fmt.Errorf("method block not found for %s %s", patch.path, patch.method)
	}

	responses, ok := mappingValue(methodNode, "responses")
	if !ok {
		return fmt.Errorf("responses block not found for %s %s", patch.path, patch.method)
	}

	foundMedia := false
	for i := 1; i < len(responses.Content); i += 2 {
		responseNode := responses.Content[i]
		contentNode, ok := mappingValue(responseNode, "content")
		if !ok {
			continue
		}
		mediaNode, ok := mappingValue(contentNode, patch.mediaType)
		if !ok {
			continue
		}
		setMappingBool(mediaNode, patch.fieldKey, patch.fieldBool)
		foundMedia = true
	}

	if !foundMedia {
		return fmt.Errorf("media type block not found for %s %s %s", patch.path, patch.method, patch.mediaType)
	}

	return nil
}

func documentMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("invalid yaml document")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("root is not a mapping")
	}
	return root, nil
}

func mappingValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1], true
		}
	}
	return nil, false
}

func setMappingBool(node *yaml.Node, key string, val bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	boolValue := "false"
	if val {
		boolValue = "true"
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1].Kind = yaml.ScalarNode
			node.Content[i+1].Tag = "!!bool"
			node.Content[i+1].Value = boolValue
			return
		}
	}
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: boolValue},
	)
}

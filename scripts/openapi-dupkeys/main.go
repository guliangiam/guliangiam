// openapi-dupkeys: 用与运行时同一个解析器（gopkg.in/yaml.v3）检测 OpenAPI 文档中的重复映射键。
//
// 背景：`openapi/embed.go` 用 yaml.v3 把文档解码成 map，重复键会让解码直接失败，
// 进而使 GET /openapi/openapi.json 恒 500；而 PyYAML 对重复键静默保留后者，
// 工具链与运行时语义不一致会形成门禁盲区。这里遍历 yaml.Node 的 MappingNode
// 逐 mapping 检测重复键，并给出重复键所在的完整路径。
//
// 用法: go run ./scripts/openapi-dupkeys [path]   （默认 openapi/openapi.yaml）
// 退出码: 0 无重复键；1 存在重复键或读取/解析失败。
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	path := "openapi/openapi.yaml"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi-dupkeys: read %s: %v\n", path, err)
		os.Exit(1)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		fmt.Fprintf(os.Stderr, "openapi-dupkeys: parse %s: %v\n", path, err)
		os.Exit(1)
	}

	var (
		duplicates []duplicateKey
		mappings   int
	)
	if len(root.Content) > 0 {
		walk(root.Content[0], "", &duplicates, &mappings)
	}

	if len(duplicates) > 0 {
		fmt.Printf("OpenAPI duplicate-key check FAILED: %d duplicate key(s) in %s\n", len(duplicates), path)
		for _, dup := range duplicates {
			location := dup.path
			if location == "" {
				location = "<root>"
			}
			fmt.Printf("  - %s: duplicate key %q at line %d (first defined at line %d)\n",
				location, dup.key, dup.line, dup.firstLine)
		}
		os.Exit(1)
	}

	fmt.Printf("OpenAPI duplicate-key check OK: %s, %d mappings, 0 duplicates\n", path, mappings)
}

type duplicateKey struct {
	path      string // 重复键所在 mapping 的路径，例如 paths./x.post.responses
	key       string
	line      int
	firstLine int
}

// walk 递归遍历节点。只跟踪 mapping 的标量键：OpenAPI 文档里不存在复杂键，
// 对非标量键跳过重复判定以免误报；alias 节点不再展开，避免重复遍历与环。
func walk(node *yaml.Node, path string, duplicates *[]duplicateKey, mappings *int) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			walk(child, path, duplicates, mappings)
		}
	case yaml.MappingNode:
		*mappings++
		seen := make(map[string]int, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode, valueNode := node.Content[i], node.Content[i+1]
			if keyNode.Kind == yaml.ScalarNode {
				key := keyNode.Value
				if firstLine, ok := seen[key]; ok {
					*duplicates = append(*duplicates, duplicateKey{
						path:      path,
						key:       key,
						line:      keyNode.Line,
						firstLine: firstLine,
					})
				} else {
					seen[key] = keyNode.Line
				}
			}
			walk(valueNode, joinPath(path, keyNode.Value), duplicates, mappings)
		}
	case yaml.SequenceNode:
		for index, child := range node.Content {
			walk(child, fmt.Sprintf("%s[%d]", path, index), duplicates, mappings)
		}
	}
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

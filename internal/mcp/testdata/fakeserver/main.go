package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// Minimal MCP server for testing (stdio transport)
// Implements: initialize, tools/list, tools/call with a simple "echo" tool

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req map[string]interface{}
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		method, ok := req["method"].(string)
		if !ok {
			continue
		}

		id, _ := req["id"].(float64)

		switch method {
		case "initialize":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      int(id),
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "fakeserver",
						"version": "0.1.0",
					},
				},
			}
			respBytes, _ := json.Marshal(resp)
			fmt.Println(string(respBytes))

		case "tools/list":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      int(id),
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "echo",
							"description": "Echo tool: repeats back the input text",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"text": map[string]interface{}{
										"type":        "string",
										"description": "Text to echo",
									},
								},
								"required": []string{"text"},
							},
						},
						{
							"name":        "reverse",
							"description": "Reverse tool: reverses the input text",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"text": map[string]interface{}{
										"type":        "string",
										"description": "Text to reverse",
									},
								},
								"required": []string{"text"},
							},
						},
					},
				},
			}
			respBytes, _ := json.Marshal(resp)
			fmt.Println(string(respBytes))

		case "tools/call":
			params, ok := req["params"].(map[string]interface{})
			if !ok {
				continue
			}

			toolName, _ := params["name"].(string)
			argsRaw, _ := params["arguments"].(json.RawMessage)

			var args map[string]interface{}
			json.Unmarshal(argsRaw, &args)
			text, _ := args["text"].(string)

			var result string
			switch toolName {
			case "echo":
				result = text
			case "reverse":
				runes := []rune(text)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				result = string(runes)
			default:
				result = "unknown tool"
			}

			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      int(id),
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": result,
						},
					},
					"isError": false,
				},
			}
			respBytes, _ := json.Marshal(resp)
			fmt.Println(string(respBytes))
		}
	}
}

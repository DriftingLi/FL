package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"forklift-training/internal/valuation"
)

func main() {
	input := flag.String("input", "", "包含完整参数、目录与系数快照的 JSON 文件")
	flag.Parse()
	if err := run(*input, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path string, output io.Writer) error {
	if path == "" {
		return fmt.Errorf("缺少 -input 参数")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取回放快照失败: %w", err)
	}
	var input valuation.ReplayInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("解析回放快照失败: %w", err)
	}
	result, err := valuation.Replay(context.Background(), input)
	if err != nil {
		return fmt.Errorf("离线回放失败: %w", err)
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

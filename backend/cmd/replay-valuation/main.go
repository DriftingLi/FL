package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"forklift-training/internal/valuation"
)

type replayOptions struct {
	Input   string
	Trials  string
	AppEnv  string
	Details bool
}

func main() {
	input := flag.String("input", "", "包含完整参数、目录、系数与试算策略快照的 JSON 文件")
	trials := flag.String("trials", "", "试算遮挡用例 JSONL；提供后全批共享 -input 快照并输出 20/30/40% 分组指标")
	appEnv := flag.String("app-env", "development", "试算策略环境；production 显式禁止 demo")
	details := flag.Bool("details", false, "批量回放是否附带逐条结果")
	flag.Parse()
	if err := runWithOptions(replayOptions{Input: *input, Trials: *trials, AppEnv: *appEnv, Details: *details}, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runWithOptions(options replayOptions, output io.Writer) error {
	if options.Input == "" {
		return fmt.Errorf("缺少 -input 参数")
	}
	data, err := os.ReadFile(options.Input)
	if err != nil {
		return fmt.Errorf("读取回放快照失败: %w", err)
	}
	var input valuation.ReplayInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("解析回放快照失败: %w", err)
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if options.Trials == "" {
		result, err := valuation.Replay(context.Background(), input)
		if err != nil {
			return fmt.Errorf("离线回放失败: %w", err)
		}
		return encoder.Encode(result)
	}
	cases, err := readTrialCases(options.Trials)
	if err != nil {
		return err
	}
	snapshot, err := valuation.NewTrialSnapshot(input)
	if err != nil {
		return fmt.Errorf("冻结批量试算快照失败: %w", err)
	}
	report, err := valuation.ReplayTrialBatch(context.Background(), snapshot, cases, options.AppEnv)
	if err != nil {
		return fmt.Errorf("批量试算回放失败: %w", err)
	}
	if !options.Details {
		report.Results = nil
	}
	return encoder.Encode(report)
}

func readTrialCases(path string) ([]valuation.TrialReplayCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("读取试算 JSONL 失败: %w", err)
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	cases := make([]valuation.TrialReplayCase, 0)
	line := 0
	for scanner.Scan() {
		line++
		payload := bytes.TrimSpace(scanner.Bytes())
		if len(payload) == 0 {
			continue
		}
		var testCase valuation.TrialReplayCase
		if err := json.Unmarshal(payload, &testCase); err != nil {
			return nil, fmt.Errorf("解析试算 JSONL 第 %d 行失败: %w", line, err)
		}
		cases = append(cases, testCase)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("遍历试算 JSONL 失败: %w", err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("试算 JSONL 没有有效用例")
	}
	return cases, nil
}

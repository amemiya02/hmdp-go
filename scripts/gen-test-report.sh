#!/usr/bin/env bash

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPORT="$PROJECT_ROOT/docs/test-report.md"
UNIT_LOG="$(mktemp -t hmdp-unit.XXXXXX)"
LOAD_LOG="$(mktemp -t hmdp-load.XXXXXX)"

cleanup() {
  status=$?
  rm -f "$UNIT_LOG" "$LOAD_LOG"
  if (( status != 0 )); then
    rm -f "$REPORT"
  fi
  exit "$status"
}
trap cleanup EXIT

cd "$PROJECT_ROOT"

echo "=== 运行集成测试 ==="
go test -p=1 -v -count=1 -timeout 180s -tags=integration \
  ./internal/util/ \
  ./internal/service/seckill/ \
  ./internal/service/order/ \
  ./internal/service/ | tee "$UNIT_LOG"

echo "=== 运行接入路径压测 ==="
go test -v -count=1 -timeout 600s -tags=load \
  -run '^TestLoadTest' \
  ./internal/service/ | tee "$LOAD_LOG"

unit_pass="$(grep -c '^--- PASS' "$UNIT_LOG" || true)"
unit_fail="$(grep -c '^--- FAIL' "$UNIT_LOG" || true)"

{
  echo "# 秒杀模块测试报告"
  echo
  echo "> 生成时间: $(date '+%Y-%m-%d %H:%M:%S')"
  echo
  echo "## 集成测试"
  echo
  echo "| 通过 | 失败 |"
  echo "|---:|---:|"
  echo "| $unit_pass | $unit_fail |"
  echo
  echo "## 接入路径压测输出"
  echo
  echo "> V2/V3 使用 Channel 或 fake Kafka writer，只统计应用内接入路径，不代表 broker 或异步落库吞吐。"
  echo
  echo '```text'
  sed -n '/=== RUN/,$p' "$LOAD_LOG"
  echo '```'
} > "$REPORT"

echo "报告已生成: $REPORT"

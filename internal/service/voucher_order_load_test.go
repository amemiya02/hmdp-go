package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amemiya02/hmdp-go/config"
	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/segmentio/kafka-go"
)

// LoadTestResult 压测结果
type LoadTestResult struct {
	TotalRequests   int64
	SuccessCount    int64
	FailCount       int64
	TotalDuration   time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	AvgLatency      time.Duration
	P50             time.Duration
	P90             time.Duration
	P95             time.Duration
	P99             time.Duration
	QPS             float64
	ThroughputMB    float64
	ErrorBreakdown  map[string]int64
	Concurrency     int
	Stock           int
}

func (r *LoadTestResult) String() string {
	return fmt.Sprintf(
		"并发=%d | 库存=%d | 总请求=%d | 成功=%d | 失败=%d | QPS=%.1f | 耗时=%v\n"+
			"延迟: min=%v avg=%v max=%v\n"+
			"P50=%v P90=%v P95=%v P99=%v",
		r.Concurrency, r.Stock, r.TotalRequests, r.SuccessCount, r.FailCount,
		r.QPS, r.TotalDuration.Round(time.Millisecond),
		r.MinLatency.Round(time.Microsecond), r.AvgLatency.Round(time.Microsecond), r.MaxLatency.Round(time.Microsecond),
		r.P50.Round(time.Microsecond), r.P90.Round(time.Microsecond), r.P95.Round(time.Microsecond), r.P99.Round(time.Microsecond),
	)
}

// runLoadTest 通用压测框架
func runLoadTest(t *testing.T, testName string, stock int, concurrency int, totalRequests int, version string) *LoadTestResult {
	vid := uint64(time.Now().UnixNano() % 10000000)
	setupSeckillData(t, vid, stock)
	defer cleanupSeckillData(vid)

	svc := NewVoucherOrderService()

	var fn func(ctx context.Context, voucherId uint64) *dto.Result
	switch version {
	case "V1":
		fn = svc.SeckillVoucherV1
	case "V2":
		fn = svc.SeckillVoucherV2
	case "V3":
		fn = svc.SeckillVoucherV3
	}

	// 结果收集
	var (
		successCount int64
		failCount    int64
		latencies    = make([]time.Duration, 0, totalRequests)
		mu           sync.Mutex
		wg           sync.WaitGroup
		errBreakdown = make(map[string]int64)
	)

	semaphore := make(chan struct{}, concurrency)
	startTime := time.Now()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		userId := uint64(800000 + i)
		go func(uid uint64) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ctx := setUserContext(uid)
			reqStart := time.Now()
			result := fn(ctx, vid)
			latency := time.Since(reqStart)

			mu.Lock()
			latencies = append(latencies, latency)
			if result.Success {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
				errBreakdown[result.ErrorMsg]++
			}
			mu.Unlock()
		}(userId)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// 计算延迟统计
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}

	result := &LoadTestResult{
		TotalRequests:  int64(totalRequests),
		SuccessCount:   successCount,
		FailCount:      failCount,
		TotalDuration:  totalDuration,
		MinLatency:     latencies[0],
		MaxLatency:     latencies[len(latencies)-1],
		AvgLatency:     totalLatency / time.Duration(len(latencies)),
		P50:            percentile(latencies, 50),
		P90:            percentile(latencies, 90),
		P95:            percentile(latencies, 95),
		P99:            percentile(latencies, 99),
		QPS:            float64(totalRequests) / totalDuration.Seconds(),
		ErrorBreakdown: errBreakdown,
		Concurrency:    concurrency,
		Stock:          stock,
	}

	// 确保无超卖
	dbOrders := getDBOrderCount(vid)
	if int(dbOrders) > stock {
		t.Errorf("OVERSELL! stock=%d but DB orders=%d", stock, dbOrders)
	}

	// 对 V2 消费 channel
	if version == "V2" {
		consumed := 0
		for {
			select {
			case <-svc.ChannelExec.Tasks:
				consumed++
			default:
				goto done
			}
		}
	done:
		t.Logf("V2 channel consumed: %d", consumed)
	}

	return result
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(p)/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

// ─────────────────────── 压测：不同并发级别 ───────────────────────

func TestLoadTest_V1_Scalability(t *testing.T) {
	stock := 200
	concurrencies := []int{10, 50, 100, 200, 500}
	totalRequests := 500

	t.Logf("=== V1 (Sync) 压测: 库存=%d, 总请求=%d ===", stock, totalRequests)
	t.Logf("%-10s %-10s %-10s %-12s %-12s %-12s %-12s %-12s %-12s %-12s",
		"并发", "成功", "失败", "QPS", "总耗时", "P50", "P90", "P95", "P99", "Max")

	for _, c := range concurrencies {
		r := runLoadTest(t, fmt.Sprintf("V1_C%d", c), stock, c, totalRequests, "V1")
		t.Logf("%-10d %-10d %-10d %-12.1f %-12v %-12v %-12v %-12v %-12v %-12v",
			r.Concurrency, r.SuccessCount, r.FailCount, r.QPS,
			r.TotalDuration.Round(time.Millisecond),
			r.P50.Round(time.Millisecond), r.P90.Round(time.Millisecond),
			r.P95.Round(time.Millisecond), r.P99.Round(time.Millisecond),
			r.MaxLatency.Round(time.Millisecond))
	}
}

func TestLoadTest_V2_Scalability(t *testing.T) {
	stock := 200
	concurrencies := []int{10, 50, 100, 200, 500}
	totalRequests := 500

	t.Logf("=== V2 (Channel) 压测: 库存=%d, 总请求=%d ===", stock, totalRequests)
	t.Logf("%-10s %-10s %-10s %-12s %-12s %-12s %-12s %-12s %-12s %-12s",
		"并发", "成功", "失败", "QPS", "总耗时", "P50", "P90", "P95", "P99", "Max")

	for _, c := range concurrencies {
		r := runLoadTest(t, fmt.Sprintf("V2_C%d", c), stock, c, totalRequests, "V2")
		t.Logf("%-10d %-10d %-10d %-12.1f %-12v %-12v %-12v %-12v %-12v %-12v",
			r.Concurrency, r.SuccessCount, r.FailCount, r.QPS,
			r.TotalDuration.Round(time.Millisecond),
			r.P50.Round(time.Millisecond), r.P90.Round(time.Millisecond),
			r.P95.Round(time.Millisecond), r.P99.Round(time.Millisecond),
			r.MaxLatency.Round(time.Millisecond))
	}
}

func TestLoadTest_V3_Scalability(t *testing.T) {
	stock := 200
	concurrencies := []int{10, 50, 100, 200, 500}
	totalRequests := 500

	t.Logf("=== V3 (Kafka) 压测: 库存=%d, 总请求=%d ===", stock, totalRequests)
	t.Logf("%-10s %-10s %-10s %-12s %-12s %-12s %-12s %-12s %-12s %-12s",
		"并发", "成功", "失败", "QPS", "总耗时", "P50", "P90", "P95", "P99", "Max")

	for _, c := range concurrencies {
		r := runLoadTest(t, fmt.Sprintf("V3_C%d", c), stock, c, totalRequests, "V3")
		t.Logf("%-10d %-10d %-10d %-12.1f %-12v %-12v %-12v %-12v %-12v %-12v",
			r.Concurrency, r.SuccessCount, r.FailCount, r.QPS,
			r.TotalDuration.Round(time.Millisecond),
			r.P50.Round(time.Millisecond), r.P90.Round(time.Millisecond),
			r.P95.Round(time.Millisecond), r.P99.Round(time.Millisecond),
			r.MaxLatency.Round(time.Millisecond))
	}
}

// ─────────────────────── 压测：版本对比（相同条件） ───────────────────────

func TestLoadTest_VersionComparison(t *testing.T) {
	stock := 100
	concurrency := 100
	totalRequests := 300

	versions := []string{"V1", "V2", "V3"}
	t.Logf("=== 版本对比: 库存=%d, 并发=%d, 总请求=%d ===", stock, concurrency, totalRequests)
	t.Logf("%-8s %-10s %-10s %-12s %-12s %-12s %-12s %-12s %-12s",
		"版本", "成功", "失败", "QPS", "总耗时", "P50", "P90", "P95", "P99")

	for _, v := range versions {
		r := runLoadTest(t, fmt.Sprintf("Compare_%s", v), stock, concurrency, totalRequests, v)
		t.Logf("%-8s %-10d %-10d %-12.1f %-12v %-12v %-12v %-12v %-12v",
			v, r.SuccessCount, r.FailCount, r.QPS,
			r.TotalDuration.Round(time.Millisecond),
			r.P50.Round(time.Millisecond), r.P90.Round(time.Millisecond),
			r.P95.Round(time.Millisecond), r.P99.Round(time.Millisecond))
	}
}

// ─────────────────────── 压测：极限 QPS ───────────────────────

func TestLoadTest_V2_HighQPS(t *testing.T) {
	stock := 1000
	concurrency := 500
	totalRequests := 2000

	r := runLoadTest(t, "V2_HighQPS", stock, concurrency, totalRequests, "V2")

	t.Logf("=== V2 极限压测 ===")
	t.Log(r.String())
	t.Logf("成功率: %.2f%%", float64(r.SuccessCount)/float64(r.TotalRequests)*100)
	t.Logf("错误分布:")
	for msg, count := range r.ErrorBreakdown {
		t.Logf("  %s: %d", msg, count)
	}
}

func TestLoadTest_V3_HighQPS(t *testing.T) {
	stock := 1000
	concurrency := 500
	totalRequests := 2000

	r := runLoadTest(t, "V3_HighQPS", stock, concurrency, totalRequests, "V3")

	t.Logf("=== V3 极限压测 ===")
	t.Log(r.String())
	t.Logf("成功率: %.2f%%", float64(r.SuccessCount)/float64(r.TotalRequests)*100)
	t.Logf("错误分布:")
	for msg, count := range r.ErrorBreakdown {
		t.Logf("  %s: %d", msg, count)
	}
}

// ─────────────────────── 压测：V3 同步模式对比 ───────────────────────

// withSyncKafka 临时切换为同步 Kafka Writer 执行测试，执行后恢复
func withSyncKafka(fn func()) {
	syncWriter := &kafka.Writer{
		Addr:     kafka.TCP(config.GlobalConfig.Kafka.Brokers...),
		Topic:    config.GlobalConfig.Kafka.Topic,
		Balancer: &kafka.LeastBytes{},
		Async:    false,
	}
	origWriter := global.KafkaWriter
	global.KafkaWriter = syncWriter
	defer func() {
		global.KafkaWriter = origWriter
		syncWriter.Close()
	}()
	fn()
}

func TestLoadTest_V3_Sync_Scalability(t *testing.T) {
	stock := 200
	concurrencies := []int{10, 50, 100, 200, 500}
	totalRequests := 500

	t.Logf("=== V3_Sync (Kafka 同步) 压测: 库存=%d, 总请求=%d ===", stock, totalRequests)
	t.Logf("%-10s %-10s %-10s %-12s %-12s %-12s %-12s %-12s %-12s %-12s",
		"并发", "成功", "失败", "QPS", "总耗时", "P50", "P90", "P95", "P99", "Max")

	for _, c := range concurrencies {
		var r *LoadTestResult
		withSyncKafka(func() {
			r = runLoadTest(t, fmt.Sprintf("V3_Sync_C%d", c), stock, c, totalRequests, "V3")
		})
		t.Logf("%-10d %-10d %-10d %-12.1f %-12v %-12v %-12v %-12v %-12v %-12v",
			r.Concurrency, r.SuccessCount, r.FailCount, r.QPS,
			r.TotalDuration.Round(time.Millisecond),
			r.P50.Round(time.Millisecond), r.P90.Round(time.Millisecond),
			r.P95.Round(time.Millisecond), r.P99.Round(time.Millisecond),
			r.MaxLatency.Round(time.Millisecond))
	}
}

func TestLoadTest_V3_Sync_HighQPS(t *testing.T) {
	stock := 1000
	concurrency := 500
	totalRequests := 2000

	var r *LoadTestResult
	withSyncKafka(func() {
		r = runLoadTest(t, "V3_Sync_HighQPS", stock, concurrency, totalRequests, "V3")
	})

	t.Logf("=== V3_Sync 极限压测 ===")
	t.Log(r.String())
	t.Logf("成功率: %.2f%%", float64(r.SuccessCount)/float64(r.TotalRequests)*100)
	t.Logf("错误分布:")
	for msg, count := range r.ErrorBreakdown {
		t.Logf("  %s: %d", msg, count)
	}
}

// ─────────────────────── 压测：持续稳定性 ───────────────────────

func TestLoadTest_SustainedLoad(t *testing.T) {
	stock := 500
	concurrency := 100
	duration := 5 * time.Second

	vid := uint64(time.Now().UnixNano() % 10000000)
	setupSeckillData(t, vid, stock)
	defer cleanupSeckillData(vid)

	svc := NewVoucherOrderService()

	var (
		successCount int64
		failCount    int64
		latencies    []time.Duration
		mu           sync.Mutex
		wg           sync.WaitGroup
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	semaphore := make(chan struct{}, concurrency)
	startTime := time.Now()

	// 持续发送请求直到超时
	requestId := 0
	for {
		select {
		case <-ctx.Done():
			goto wait
		default:
		}

		wg.Add(1)
		requestId++
		userId := uint64(900000 + requestId)
		go func(uid uint64) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			reqCtx := setUserContext(uid)
			reqStart := time.Now()
			result := svc.SeckillVoucherV2(reqCtx, vid)
			latency := time.Since(reqStart)

			mu.Lock()
			latencies = append(latencies, latency)
			if result.Success {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
			}
			mu.Unlock()
		}(userId)
	}

wait:
	wg.Wait()
	totalDuration := time.Since(startTime)

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}

	totalRequests := successCount + failCount
	t.Logf("=== V2 持续压测 (持续 %v) ===", duration)
	t.Logf("并发=%d | 总请求=%d | 成功=%d | 失败=%d | QPS=%.1f",
		concurrency, totalRequests, successCount, failCount,
		float64(totalRequests)/totalDuration.Seconds())
	t.Logf("延迟: min=%v avg=%v max=%v",
		latencies[0].Round(time.Microsecond),
		(totalLatency / time.Duration(len(latencies))).Round(time.Microsecond),
		latencies[len(latencies)-1].Round(time.Microsecond))
	t.Logf("P50=%v P90=%v P95=%v P99=%v",
		percentile(latencies, 50).Round(time.Microsecond),
		percentile(latencies, 90).Round(time.Microsecond),
		percentile(latencies, 95).Round(time.Microsecond),
		percentile(latencies, 99).Round(time.Microsecond))
	t.Logf("成功率: %.2f%%", float64(successCount)/float64(totalRequests)*100)

	// 清理 channel
	for {
		select {
		case <-svc.ChannelExec.Tasks:
		default:
			goto done
		}
	}
done:
}

// getDBOrderCount 已在 voucher_order_test.go 中定义

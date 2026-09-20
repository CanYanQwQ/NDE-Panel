package traffic

import "testing"

// burst 决定高延迟链路上的吞吐天花板:桶空了就得等下一批令牌,
// 所以实际吞吐大约是 burst/RTT。这组用例钉住的是「桶必须覆盖 BDP」,
// 曾经为了让短时测速数字好看把桶封顶在 256KB,结果限速 100Mbps 的
// 跨国中转只跑得到十几兆,体感一顿一顿的。
func TestBurstCoversBandwidthDelayProduct(t *testing.T) {
	cases := []struct {
		name      string
		rate      int // 字节/秒
		rttMillis int
		wantMbps  float64 // 至少要能跑到这个速度
	}{
		{"100Mbps 跨国 150ms", 12_500_000, 150, 90},
		{"50Mbps 跨国 200ms", 6_250_000, 200, 45},
		{"20Mbps 国内 50ms", 2_500_000, 50, 18},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			burst := burstOf(c.rate)
			// 一个 RTT 里最多送出一桶,所以吞吐上限 = burst / RTT
			gotMbps := float64(burst) * 8 / (float64(c.rttMillis) / 1000) / 1e6
			if gotMbps < c.wantMbps {
				t.Fatalf("burst %d 字节在 %dms RTT 下只能跑 %.1f Mbps,达不到限速要求的 %.0f Mbps",
					burst, c.rttMillis, gotMbps, c.wantMbps)
			}
		})
	}
}

// 桶不能比速率本身还大,否则限速形同虚设 —— 第一秒就能把一整秒的量放完。
func TestBurstNeverExceedsRate(t *testing.T) {
	for _, r := range []int{1024, 64 * 1024, 500 * 1024, 5_000_000} {
		if b := burstOf(r); b > r {
			t.Fatalf("rate %d 的桶是 %d,比速率还大", r, b)
		}
	}
}

// 桶太小会把单次读写切碎,反而拖垮吞吐(常见缓冲是 32~64KB)。
func TestBurstStaysAboveIoBufferSize(t *testing.T) {
	// 只对「速率本身够大」的情况成立:限速定得比缓冲还小时,桶只能跟着速率走。
	for _, r := range []int{1_000_000, 12_500_000} {
		if b := burstOf(r); b < 64*1024 {
			t.Fatalf("rate %d 的桶只有 %d 字节,比常见 IO 缓冲还小", r, b)
		}
	}
}

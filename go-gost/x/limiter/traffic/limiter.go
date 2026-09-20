package traffic

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	limiter "github.com/go-gost/core/limiter/traffic"
	"golang.org/x/time/rate"
)

type llimiter struct {
	limiter *rate.Limiter
}

// 突发额度(令牌桶容量)。
//
// 这个值必须覆盖链路的 BDP(带宽 × 时延),否则吞吐会被死死卡在 burst/RTT ——
// 桶空了就等、补满再冲,体感就是一顿一顿的「卡」。曾经为了让短时测速的数字
// 贴近设定值,把桶压到 1/8 速率、且封顶 256KB,代价是:
//
//   限速 100Mbps、RTT 150ms 的跨国中转 → 256KB/0.15s ≈ 13Mbps
//
// 也就是限速设多少都没用,实际只跑得到十几兆。测速数字好看是观感,跑不动是
// 真问题,所以现在按速率比例给桶,并且不再设那个死上限。
//
// 取 1/2 秒的量:
//   - 100Mbps → 6.25MB,足够覆盖 500ms 以内的 RTT
//   - 短时测速(通常 3 秒)偏高约 15%,可以接受
// 下限 256KB:低于这个值,单次读写(常见 32KB~64KB 缓冲)会被切得太碎,
// 反而拖垮吞吐。上限 8MB 纯粹是防呆,挡住有人把限速填成天文数字。
const (
	burstDivisor = 2
	minBurst     = 256 * 1024
	maxBurst     = 8 * 1024 * 1024
)

func burstOf(r int) int {
	b := r / burstDivisor
	if b > maxBurst {
		b = maxBurst
	}
	if b < minBurst {
		b = minBurst
	}
	if b > r {
		b = r // 限速本身就很小时,桶不能比速率还大
	}
	return b
}

func NewLimiter(r int) limiter.Limiter {
	return &llimiter{
		limiter: rate.NewLimiter(rate.Limit(r), burstOf(r)),
	}
}

func (l *llimiter) Wait(ctx context.Context, n int) int {
	if l.limiter.Burst() < n {
		n = l.limiter.Burst()
	}
	l.limiter.WaitN(ctx, n)
	return n
}

func (l *llimiter) Limit() int {
	return int(l.limiter.Limit())
}

func (l *llimiter) Set(n int) {
	l.limiter.SetLimit(rate.Limit(n))
	l.limiter.SetBurst(burstOf(n))
}

func (l *llimiter) String() string {
	return strconv.Itoa(int(l.limiter.Limit()))
}

type limiterGroup struct {
	limiters []limiter.Limiter
}

func newLimiterGroup(limiters ...limiter.Limiter) *limiterGroup {
	sort.Slice(limiters, func(i, j int) bool {
		return limiters[i].Limit() < limiters[j].Limit()
	})
	return &limiterGroup{limiters: limiters}
}

func (l *limiterGroup) Wait(ctx context.Context, n int) int {
	for i := range l.limiters {
		if v := l.limiters[i].Wait(ctx, n); v < n {
			n = v
		}
	}
	return n
}

func (l *limiterGroup) Limit() int {
	if len(l.limiters) == 0 {
		return 0
	}

	return l.limiters[0].Limit()
}

func (l *limiterGroup) Set(n int) {}

func (l *limiterGroup) String() string {
	return fmt.Sprintf("%v", l.limiters)
}

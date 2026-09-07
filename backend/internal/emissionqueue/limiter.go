package emissionqueue

import (
	"sync"
	"time"
)

type tenantBucket struct {
	tokens float64
	last   time.Time
}

// TenantRateLimiter is a token bucket per tenant. TryAcquire never blocks a
// River worker: when the bucket is empty the job is snoozed, freeing the slot
// so another tenant can progress.
type TenantRateLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	now     func() time.Time
	buckets map[string]tenantBucket
}

func NewTenantRateLimiter(ratePerSecond float64, burst int) *TenantRateLimiter {
	if ratePerSecond <= 0 {
		ratePerSecond = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &TenantRateLimiter{
		rate: ratePerSecond, burst: float64(burst), now: timeNow(), buckets: map[string]tenantBucket{},
	}
}

func (l *TenantRateLimiter) TryAcquire(tenantID string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	bucket, ok := l.buckets[tenantID]
	if !ok {
		bucket = tenantBucket{tokens: l.burst, last: now}
	}
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens = min(l.burst, bucket.tokens+elapsed*l.rate)
		bucket.last = now
	}
	if bucket.tokens >= 1 {
		bucket.tokens--
		l.buckets[tenantID] = bucket
		return true, 0
	}

	l.buckets[tenantID] = bucket
	wait := time.Duration(((1 - bucket.tokens) / l.rate) * float64(time.Second))
	if wait < time.Millisecond {
		wait = time.Millisecond
	}
	return false, wait
}

func timeNow() func() time.Time {
	return func() time.Time { return time.Now().UTC() }
}

package emissionqueue

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTenantRateLimiterAislaBuckets(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	limiter := NewTenantRateLimiter(1, 1)
	limiter.now = func() time.Time { return now }

	if allowed, _ := limiter.TryAcquire("tenant-a"); !allowed {
		t.Fatal("el primer token del tenant A debe estar disponible")
	}
	if allowed, retryAfter := limiter.TryAcquire("tenant-a"); allowed || retryAfter != time.Second {
		t.Fatalf("segundo token A: allowed=%v retry=%s", allowed, retryAfter)
	}
	if allowed, _ := limiter.TryAcquire("tenant-b"); !allowed {
		t.Fatal("tenant B no debe ser bloqueado por el consumo de tenant A")
	}

	now = now.Add(time.Second)
	if allowed, _ := limiter.TryAcquire("tenant-a"); !allowed {
		t.Fatal("tenant A debe recuperar un token después de un segundo")
	}
}

func TestTenantRateLimiterConcurrenteNoComparteCuota(t *testing.T) {
	const tenants = 32
	limiter := NewTenantRateLimiter(1, 1)
	start := make(chan struct{})
	allowed := make(chan string, tenants*2)
	var wg sync.WaitGroup
	for tenantIndex := range tenants {
		tenant := fmt.Sprintf("tenant-%d", tenantIndex)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if ok, _ := limiter.TryAcquire(tenant); ok {
					allowed <- tenant
				}
			}()
		}
	}
	close(start)
	wg.Wait()
	close(allowed)
	counts := make(map[string]int, tenants)
	for tenant := range allowed {
		counts[tenant]++
	}
	if len(counts) != tenants {
		t.Fatalf("solo %d/%d tenants obtuvieron su cuota", len(counts), tenants)
	}
	for tenant, count := range counts {
		if count != 1 {
			t.Fatalf("%s obtuvo %d tokens; se esperaba exactamente uno", tenant, count)
		}
	}
}

func TestCircuitBreakerAislaTenantYAdmiteUnaSonda(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	breaker := NewCircuitBreaker(2, 30*time.Second)
	breaker.now = func() time.Time { return now }

	breaker.Failure("tenant-a")
	breaker.Failure("tenant-a")
	if allowed, retryAfter := breaker.Allow("tenant-a"); allowed || retryAfter != 30*time.Second {
		t.Fatalf("circuito A: allowed=%v retry=%s", allowed, retryAfter)
	}
	if allowed, _ := breaker.Allow("tenant-b"); !allowed {
		t.Fatal("la falla de A no debe abrir el circuito de B")
	}

	now = now.Add(30 * time.Second)
	if allowed, _ := breaker.Allow("tenant-a"); !allowed {
		t.Fatal("se esperaba una sonda half-open")
	}
	if allowed, _ := breaker.Allow("tenant-a"); allowed {
		t.Fatal("solo una sonda half-open puede ejecutarse")
	}
	breaker.AbortProbe("tenant-a")
	if allowed, _ := breaker.Allow("tenant-a"); !allowed {
		t.Fatal("una sonda abortada debe liberar el estado half-open")
	}
	breaker.Success("tenant-a")
	if allowed, _ := breaker.Allow("tenant-a"); !allowed {
		t.Fatal("el éxito debe cerrar el circuito")
	}
}

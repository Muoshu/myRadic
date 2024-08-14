package index_service

import (
	"math/rand"
	"sync/atomic"
)

type LoadBalancer interface {
	Take([]string) string
}

type RoundRobin struct {
	acc int64
}

func (rr *RoundRobin) Take(endPoints []string) string {
	if len(endPoints) == 0 {
		return ""
	}
	n := atomic.AddInt64(&rr.acc, 1)
	index := int(n % int64(len(endPoints)))
	return endPoints[index]
}

type RandomSelect struct {
}

func (b *RandomSelect) Take(endpoints []string) string {
	if len(endpoints) == 0 {
		return ""
	}
	index := rand.Intn(len(endpoints)) // 随机选择
	return endpoints[index]
}

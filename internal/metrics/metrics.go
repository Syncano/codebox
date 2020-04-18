package metrics

import (
	"sync/atomic"

	"go.opencensus.io/metric"
)

type Int64GaugeEntry struct {
	*metric.Int64GaugeEntry
	val int64
}

func (g *Int64GaugeEntry) Add(v int64) {
	atomic.AddInt64(&g.val, v)
	g.Int64GaugeEntry.Add(v)
}

func (g *Int64GaugeEntry) Set(v int64) {
	atomic.StoreInt64(&g.val, v)
	g.Int64GaugeEntry.Set(v)
}

func (g *Int64GaugeEntry) Value() int64 {
	return atomic.LoadInt64(&g.val)
}

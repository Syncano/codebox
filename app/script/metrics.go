package script

import (
	"sync"

	"go.opencensus.io/metric"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricproducer"

	"github.com/Syncano/codebox/internal/metrics"
	"github.com/Syncano/pkg-go/util"
)

var (
	initOnceMetrics sync.Once
	metr            *MetricsData
)

type MetricsData struct {
	workerCPU *metrics.Int64GaugeEntry
}

func Metrics() *MetricsData {
	initOnceMetrics.Do(func() {
		r := metric.NewRegistry()
		metricproducer.GlobalManager().AddProducer(r)

		cpuGauge, err := r.AddInt64Gauge(
			"codebox/worker/cpu",
			metric.WithDescription("Codebox worker free mcpu."),
			metric.WithUnit(metricdata.UnitDimensionless),
		)
		util.Must(err)

		cpuGaugeEntry, err := cpuGauge.GetEntry()
		util.Must(err)

		metr = &MetricsData{
			workerCPU: &metrics.Int64GaugeEntry{Int64GaugeEntry: cpuGaugeEntry},
		}
	})

	return metr
}

func (m *MetricsData) WorkerCPU() *metrics.Int64GaugeEntry {
	return m.workerCPU
}

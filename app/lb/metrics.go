package lb

import (
	"sync"

	"go.opencensus.io/metric"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricproducer"

	"github.com/Syncano/codebox/internal/metrics"
	"github.com/Syncano/pkg-go/v2/util"
)

var (
	initOnce sync.Once
	metr     *MetricsData
)

type MetricsData struct {
	workerCPU   *metrics.Int64GaugeEntry
	workerCount *metrics.Int64GaugeEntry
}

func Metrics() *MetricsData {
	initOnce.Do(func() {
		r := metric.NewRegistry()
		metricproducer.GlobalManager().AddProducer(r)

		cpuGauge, err := r.AddInt64Gauge(
			"codebox/worker/cpu",
			metric.WithDescription("Codebox worker total free mcpu."),
			metric.WithUnit(metricdata.UnitDimensionless),
		)
		util.Must(err)

		cpuGaugeEntry, err := cpuGauge.GetEntry()
		util.Must(err)

		countGauge, err := r.AddInt64Gauge(
			"codebox/worker/count",
			metric.WithDescription("Codebox worker total count."),
			metric.WithUnit(metricdata.UnitDimensionless),
		)
		util.Must(err)

		countGaugeEntry, err := countGauge.GetEntry()
		util.Must(err)

		metr = &MetricsData{
			workerCPU:   &metrics.Int64GaugeEntry{Int64GaugeEntry: cpuGaugeEntry},
			workerCount: &metrics.Int64GaugeEntry{Int64GaugeEntry: countGaugeEntry},
		}
	})

	return metr
}

func (m *MetricsData) WorkerCPU() *metrics.Int64GaugeEntry {
	return m.workerCPU
}

func (m *MetricsData) WorkerCount() *metrics.Int64GaugeEntry {
	return m.workerCount
}

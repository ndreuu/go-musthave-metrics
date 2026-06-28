package agent

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type Metric struct {
	MType string 
	Name  string
	Value float64
}

type Collector struct {
	mu         sync.RWMutex
	metrics    map[string]*Metric
	pollCount  int64
	randSource *rand.Rand
}

func NewCollector() *Collector {
	return &Collector{
		metrics:    make(map[string]*Metric),
		randSource: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (c *Collector) Collect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	setGauge := func(name string, value float64) {
		c.metrics[name] = &Metric{
			MType: "gauge",
			Name:  name,
			Value: value,
		}
	}

	setGauge("Alloc", float64(memStats.Alloc))
	setGauge("BuckHashSys", float64(memStats.BuckHashSys))
	setGauge("Frees", float64(memStats.Frees))
	setGauge("GCCPUFraction", memStats.GCCPUFraction)
	setGauge("GCSys", float64(memStats.GCSys))
	setGauge("HeapAlloc", float64(memStats.HeapAlloc))
	setGauge("HeapIdle", float64(memStats.HeapIdle))
	setGauge("HeapInuse", float64(memStats.HeapInuse))
	setGauge("HeapObjects", float64(memStats.HeapObjects))
	setGauge("HeapReleased", float64(memStats.HeapReleased))
	setGauge("HeapSys", float64(memStats.HeapSys))
	setGauge("LastGC", float64(memStats.LastGC))
	setGauge("Lookups", float64(memStats.Lookups))
	setGauge("MCacheInuse", float64(memStats.MCacheInuse))
	setGauge("MCacheSys", float64(memStats.MCacheSys))
	setGauge("MSpanInuse", float64(memStats.MSpanInuse))
	setGauge("MSpanSys", float64(memStats.MSpanSys))
	setGauge("Mallocs", float64(memStats.Mallocs))
	setGauge("NextGC", float64(memStats.NextGC))
	setGauge("NumForcedGC", float64(memStats.NumForcedGC))
	setGauge("NumGC", float64(memStats.NumGC))
	setGauge("OtherSys", float64(memStats.OtherSys))
	setGauge("PauseTotalNs", float64(memStats.PauseTotalNs))
	setGauge("StackInuse", float64(memStats.StackInuse))
	setGauge("StackSys", float64(memStats.StackSys))
	setGauge("Sys", float64(memStats.Sys))
	setGauge("TotalAlloc", float64(memStats.TotalAlloc))

	c.pollCount++

	c.metrics["PollCount"] = &Metric{
		MType: "counter",
		Name:  "PollCount",
		Value: float64(c.pollCount),
	}

	randomValue := c.randSource.Float64() * 1000
	c.metrics["RandomValue"] = &Metric{
		MType: "gauge",
		Name:  "RandomValue",
		Value: randomValue,
	}
}

func (c *Collector) CollectGopsutil() {
	values := make(map[string]float64)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if vmStat, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		values["TotalMemory"] = float64(vmStat.Total)
		values["FreeMemory"] = float64(vmStat.Free)
	}

	if cpuPercentages, err := cpu.PercentWithContext(ctx, 0, true); err == nil {
		for i, pct := range cpuPercentages {
			values[fmt.Sprintf("CPUutilization%d", i+1)] = pct
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for name, value := range values {
		c.metrics[name] = &Metric{
			MType: "gauge",
			Name:  name,
			Value: value,
		}
	}
}

func (c *Collector) GetMetrics() []*Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Metric, 0, len(c.metrics))
	for _, m := range c.metrics {
		result = append(result, &Metric{
			MType: m.MType,
			Name:  m.Name,
			Value: m.Value,
		})
	}
	return result
}

func (c *Collector) GetMetric(name string) *Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if m, ok := c.metrics[name]; ok {
		return &Metric{
			MType: m.MType,
			Name:  m.Name,
			Value: m.Value,
		}
	}
	return nil
}

package metrics

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and reports metrics
type MetricsCollector struct {
	requestCount         uint64
	eventCount           uint64
	deliveredEventCount  uint64
	bouncedEventCount    uint64
	responseTimes        map[string][]time.Duration // path -> response times
	responseTimesByCode  map[int][]time.Duration    // status code -> response times
	eventProcessingTimes []time.Duration            // event processing times
	batchProcessingTimes []BatchProcessingTime      // batch processing times
	mutex                sync.RWMutex
	startTime            time.Time
}

// BatchProcessingTime tracks processing time for a batch
type BatchProcessingTime struct {
	Duration time.Duration
	Size     int
}

// MetricsResponse represents the metrics API response
type MetricsResponse struct {
	Uptime                string           `json:"uptime"`
	RequestCount          uint64           `json:"request_count"`
	EventCount            uint64           `json:"event_count"`
	DeliveredEventCount   uint64           `json:"delivered_event_count"`
	BouncedEventCount     uint64           `json:"bounced_event_count"`
	AverageResponseMS     float64          `json:"average_response_ms"`
	AverageEventProcessMS float64          `json:"average_event_process_ms"`
	BatchMetrics          BatchMetrics     `json:"batch_metrics"`
	StatusCodeCounts      map[int]int      `json:"status_code_counts"`
	P95ResponseTimeMS     float64          `json:"p95_response_time_ms"`
	P95EventProcessTimeMS float64          `json:"p95_event_process_time_ms"`
	EndpointStatistics    map[string]Stats `json:"endpoint_statistics"`
}

// BatchMetrics tracks batch processing statistics
type BatchMetrics struct {
	BatchCount            int     `json:"batch_count"`
	TotalEvents           int     `json:"total_events"`
	AverageBatchSize      float64 `json:"average_batch_size"`
	AverageBatchTimeMS    float64 `json:"average_batch_time_ms"`
	EventsPerSecond       float64 `json:"events_per_second"`
	P95BatchProcessTimeMS float64 `json:"p95_batch_process_time_ms"`
}

// Stats represents statistics for an endpoint
type Stats struct {
	Count             int     `json:"count"`
	AverageResponseMS float64 `json:"average_response_ms"`
	P95ResponseTimeMS float64 `json:"p95_response_time_ms"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requestCount:         0,
		eventCount:           0,
		deliveredEventCount:  0,
		bouncedEventCount:    0,
		responseTimes:        make(map[string][]time.Duration),
		responseTimesByCode:  make(map[int][]time.Duration),
		eventProcessingTimes: make([]time.Duration, 0, 1000),
		batchProcessingTimes: make([]BatchProcessingTime, 0, 1000),
		startTime:            time.Now(),
	}
}

// IncrementRequestCount increments the request counter
func (m *MetricsCollector) IncrementRequestCount() {
	atomic.AddUint64(&m.requestCount, 1)
}

// IncrementEventCount increments the event counter
func (m *MetricsCollector) IncrementEventCount(eventType string) {
	atomic.AddUint64(&m.eventCount, 1)

	switch eventType {
	case "delivered":
		atomic.AddUint64(&m.deliveredEventCount, 1)
	case "bounced":
		atomic.AddUint64(&m.bouncedEventCount, 1)
	}
}

// RecordResponseTime records the response time for a request
func (m *MetricsCollector) RecordResponseTime(path string, statusCode int, method string) {
	duration := time.Since(m.startTime)

	// Use a key that combines method and path
	key := method + " " + path

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Store response time by path
	m.responseTimes[key] = append(m.responseTimes[key], duration)

	// Store response time by status code
	m.responseTimesByCode[statusCode] = append(m.responseTimesByCode[statusCode], duration)
}

// RecordEventProcessingTime records the processing time for an event
func (m *MetricsCollector) RecordEventProcessingTime(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.eventProcessingTimes = append(m.eventProcessingTimes, duration)

	// If we have too many samples, remove the oldest ones
	if len(m.eventProcessingTimes) > 10000 {
		m.eventProcessingTimes = m.eventProcessingTimes[1000:]
	}
}

// RecordBatchProcessingTime records processing metrics for a batch of events
func (m *MetricsCollector) RecordBatchProcessingTime(duration time.Duration, batchSize int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.batchProcessingTimes = append(m.batchProcessingTimes, BatchProcessingTime{
		Duration: duration,
		Size:     batchSize,
	})

	// If we have too many samples, remove the oldest ones
	if len(m.batchProcessingTimes) > 10000 {
		m.batchProcessingTimes = m.batchProcessingTimes[1000:]
	}
}

// WriteMetrics writes metrics to the response
func (m *MetricsCollector) WriteMetrics(w http.ResponseWriter) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Calculate overall average response time
	var totalResponseTime time.Duration
	var count int

	for _, times := range m.responseTimes {
		for _, t := range times {
			totalResponseTime += t
			count++
		}
	}

	var avgResponseTime float64
	if count > 0 {
		avgResponseTime = float64(totalResponseTime) / float64(count) / float64(time.Millisecond)
	}

	// Calculate average event processing time
	var avgEventProcessTime float64
	if len(m.eventProcessingTimes) > 0 {
		var totalEventTime time.Duration
		for _, t := range m.eventProcessingTimes {
			totalEventTime += t
		}
		avgEventProcessTime = float64(totalEventTime) / float64(len(m.eventProcessingTimes)) / float64(time.Millisecond)
	}

	// Calculate batch metrics
	batchMetrics := BatchMetrics{}
	if len(m.batchProcessingTimes) > 0 {
		batchMetrics.BatchCount = len(m.batchProcessingTimes)

		// Calculate total events and average batch size
		var totalEvents int
		var totalBatchTime time.Duration

		for _, b := range m.batchProcessingTimes {
			totalEvents += b.Size
			totalBatchTime += b.Duration
		}

		batchMetrics.TotalEvents = totalEvents
		batchMetrics.AverageBatchSize = float64(totalEvents) / float64(batchMetrics.BatchCount)
		batchMetrics.AverageBatchTimeMS = float64(totalBatchTime) / float64(batchMetrics.BatchCount) / float64(time.Millisecond)

		// Calculate events per second
		totalProcessingTimeSeconds := float64(totalBatchTime) / float64(time.Second)
		if totalProcessingTimeSeconds > 0 {
			batchMetrics.EventsPerSecond = float64(totalEvents) / totalProcessingTimeSeconds
		}

		// Calculate P95 batch processing time
		batchTimes := make([]time.Duration, len(m.batchProcessingTimes))
		for i, b := range m.batchProcessingTimes {
			batchTimes[i] = b.Duration
		}
		batchMetrics.P95BatchProcessTimeMS = calculateP95(batchTimes)
	}

	// Calculate P95 response time
	p95Time := calculateP95(flattenDurations(m.responseTimes))

	// Calculate P95 event processing time
	p95EventTime := calculateP95(m.eventProcessingTimes)

	// Calculate status code counts
	statusCodeCounts := make(map[int]int)
	for code, times := range m.responseTimesByCode {
		statusCodeCounts[code] = len(times)
	}

	// Calculate endpoint statistics
	endpointStats := make(map[string]Stats)
	for path, times := range m.responseTimes {
		endpointStats[path] = Stats{
			Count:             len(times),
			AverageResponseMS: calculateAverage(times),
			P95ResponseTimeMS: calculateP95(times),
		}
	}

	// Create response
	metrics := MetricsResponse{
		Uptime:                time.Since(m.startTime).String(),
		RequestCount:          atomic.LoadUint64(&m.requestCount),
		EventCount:            atomic.LoadUint64(&m.eventCount),
		DeliveredEventCount:   atomic.LoadUint64(&m.deliveredEventCount),
		BouncedEventCount:     atomic.LoadUint64(&m.bouncedEventCount),
		AverageResponseMS:     avgResponseTime,
		AverageEventProcessMS: avgEventProcessTime,
		BatchMetrics:          batchMetrics,
		StatusCodeCounts:      statusCodeCounts,
		P95ResponseTimeMS:     p95Time,
		P95EventProcessTimeMS: p95EventTime,
		EndpointStatistics:    endpointStats,
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// flattenDurations flattens a map of durations into a single slice
func flattenDurations(durationsMap map[string][]time.Duration) []time.Duration {
	var result []time.Duration
	for _, durations := range durationsMap {
		result = append(result, durations...)
	}
	return result
}

// calculateAverage calculates the average duration in milliseconds
func calculateAverage(durations []time.Duration) float64 {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	return float64(total) / float64(len(durations)) / float64(time.Millisecond)
}

// calculateP95 calculates the 95th percentile response time in milliseconds
func calculateP95(durations []time.Duration) float64 {
	if len(durations) == 0 {
		return 0
	}

	// Sort durations
	sortedDurations := make([]time.Duration, len(durations))
	copy(sortedDurations, durations)

	// Bubble sort for simplicity (could use a more efficient sort in production)
	for i := 0; i < len(sortedDurations); i++ {
		for j := 0; j < len(sortedDurations)-i-1; j++ {
			if sortedDurations[j] > sortedDurations[j+1] {
				sortedDurations[j], sortedDurations[j+1] = sortedDurations[j+1], sortedDurations[j]
			}
		}
	}

	// Calculate index for P95
	idx := int(float64(len(sortedDurations)) * 0.95)
	if idx >= len(sortedDurations) {
		idx = len(sortedDurations) - 1
	}

	return float64(sortedDurations[idx]) / float64(time.Millisecond)
}

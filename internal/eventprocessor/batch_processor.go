package eventprocessor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/logankrause16/email_processing/internal/domain"
	"github.com/logankrause16/email_processing/internal/repository"
	"github.com/logankrause16/email_processing/internal/service"
	"github.com/logankrause16/email_processing/pkg/eventpool"
	"github.com/logankrause16/email_processing/pkg/metrics"
)

// BatchEventProcessor handles processing events in batches from the EventPool
type BatchEventProcessor struct {
	domainService    *service.DomainService
	batchRepo        repository.BatchDomainRepository
	metricsCollector *metrics.MetricsCollector
	logger           *log.Logger
	workerCount      int
	batchSize        int
	flushInterval    time.Duration
	stopCh           chan struct{}
	wg               sync.WaitGroup
	pool             eventpool.EventPool
}

// NewBatchEventProcessor creates a new batch event processor
func NewBatchEventProcessor(
	domainService *service.DomainService,
	batchRepo repository.BatchDomainRepository,
	metricsCollector *metrics.MetricsCollector,
	logger *log.Logger,
	workerCount int,
) *BatchEventProcessor {
	return &BatchEventProcessor{
		domainService:    domainService,
		batchRepo:        batchRepo,
		metricsCollector: metricsCollector,
		logger:           logger,
		workerCount:      workerCount,
		batchSize:        100, // Default batch size
		flushInterval:    100 * time.Millisecond,
		stopCh:           make(chan struct{}),
	}
}

// Start begins processing events in batches
func (p *BatchEventProcessor) Start() {
	p.logger.Println("Starting batch event processor...")

	// Initialize the event pool
	p.pool = eventpool.SpawnEventPool()

	// Create a channel to distribute events to workers
	eventCh := make(chan *eventpool.Event, p.batchSize*p.workerCount)

	// Start worker goroutines
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.batchWorker(i, eventCh)
	}

	// Start event collector goroutine
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer close(eventCh)

		for {
			select {
			case <-p.stopCh:
				p.logger.Println("Event collector stopped")
				return
			default:
				// Get an event from the pool
				event := p.pool.GetEvent()

				// Send event to worker channel
				select {
				case eventCh <- event:
				case <-p.stopCh:
					// Return event to pool if stopping
					p.pool.RecycleEvent(event)
					p.logger.Println("Event collector stopped while sending")
					return
				}
			}
		}
	}()

	p.logger.Printf("Batch event processor started with %d workers and batch size %d",
		p.workerCount, p.batchSize)
}

// Stop halts event processing
func (p *BatchEventProcessor) Stop() {
	p.logger.Println("Stopping batch event processor...")
	close(p.stopCh)
	p.wg.Wait()
	p.pool.Close()
	p.logger.Println("Batch event processor stopped")
}

// batchWorker processes events in batches
func (p *BatchEventProcessor) batchWorker(id int, eventCh <-chan *eventpool.Event) {
	defer p.wg.Done()

	p.logger.Printf("Batch worker %d started", id)

	// Batch processing state
	batch := make([]repository.EventBatch, 0, p.batchSize)
	eventMap := make(map[string]map[domain.EventType]int)
	recycleQueue := make([]*eventpool.Event, 0, p.batchSize)

	// Timer for periodic flushing
	flushTimer := time.NewTimer(p.flushInterval)
	defer flushTimer.Stop()

	// Process and clear current batch
	processEvents := func() {
		if len(eventMap) == 0 {
			return
		}

		start := time.Now()

		// Convert map to batch
		batch = batch[:0] // Clear slice but reuse capacity
		for domainName, events := range eventMap {
			for eventType, count := range events {
				batch = append(batch, repository.EventBatch{
					DomainName: domainName,
					EventType:  eventType,
					Count:      count,
				})
			}
		}

		// Process batch
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := p.batchRepo.IncrementEventCountBatch(ctx, batch)
		cancel()

		if err != nil {
			p.logger.Printf("Worker %d error processing batch: %v", id, err)
		}

		// Record metrics
		duration := time.Since(start)
		batchSize := len(batch)
		p.metricsCollector.RecordBatchProcessingTime(duration, batchSize)

		// Clear event map for next batch
		for domain := range eventMap {
			delete(eventMap, domain)
		}
	}

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed, process remaining events
				processEvents()

				// Recycle remaining events
				for _, e := range recycleQueue {
					p.pool.RecycleEvent(e)
				}

				p.logger.Printf("Batch worker %d finished", id)
				return
			}

			// Convert event type
			var eventType domain.EventType

			if event.Type == eventpool.TypeDelivered {
				eventType = domain.EventDelivered
				p.metricsCollector.IncrementEventCount("delivered")
			} else {
				eventType = domain.EventBounced
				p.metricsCollector.IncrementEventCount("bounced")
			}

			// Add to batch
			if eventMap[event.Domain] == nil {
				eventMap[event.Domain] = make(map[domain.EventType]int)
			}
			eventMap[event.Domain][eventType]++

			// Queue event for recycling
			recycleQueue = append(recycleQueue, event)

			// Process if batch is full
			if len(eventMap) >= p.batchSize {
				processEvents()

				// Recycle events
				for _, e := range recycleQueue {
					p.pool.RecycleEvent(e)
				}
				recycleQueue = recycleQueue[:0] // Clear recycle queue

				// Reset timer
				if !flushTimer.Stop() {
					<-flushTimer.C
				}
				flushTimer.Reset(p.flushInterval)
			}

		case <-flushTimer.C:
			// Process partial batch on timeout
			if len(eventMap) > 0 {
				processEvents()

				// Recycle events
				for _, e := range recycleQueue {
					p.pool.RecycleEvent(e)
				}
				recycleQueue = recycleQueue[:0] // Clear recycle queue
			}

			flushTimer.Reset(p.flushInterval)

		case <-p.stopCh:
			// Process remaining events and exit
			processEvents()

			// Recycle remaining events
			for _, e := range recycleQueue {
				p.pool.RecycleEvent(e)
			}

			p.logger.Printf("Batch worker %d stopped", id)
			return
		}
	}
}

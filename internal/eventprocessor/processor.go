package eventprocessor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/logankrause16/email_processing/internal/domain"
	"github.com/logankrause16/email_processing/internal/service"
	"github.com/logankrause16/email_processing/pkg/eventpool"
	"github.com/logankrause16/email_processing/pkg/metrics"
)

// Processor defines the interface for event processors
type Processor interface {
	// Start begins processing events
	Start()

	// Stop halts event processing
	Stop()
}

// EventProcessor handles processing events from the EventPool
type EventProcessor struct {
	domainService    *service.DomainService
	metricsCollector *metrics.MetricsCollector
	logger           *log.Logger
	workerCount      int
	stopCh           chan struct{}
	wg               sync.WaitGroup
	pool             eventpool.EventPool
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(
	domainService *service.DomainService,
	metricsCollector *metrics.MetricsCollector,
	logger *log.Logger,
	workerCount int,
) *EventProcessor {
	return &EventProcessor{
		domainService:    domainService,
		metricsCollector: metricsCollector,
		logger:           logger,
		workerCount:      workerCount,
		stopCh:           make(chan struct{}),
	}
}

// Start begins processing events
func (p *EventProcessor) Start() {
	p.logger.Println("Starting event processor...")

	// Initialize the event pool
	p.pool = eventpool.SpawnEventPool()

	// Start worker goroutines
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	p.logger.Printf("Event processor started with %d workers", p.workerCount)
}

// Stop halts event processing
func (p *EventProcessor) Stop() {
	p.logger.Println("Stopping event processor...")
	close(p.stopCh)
	p.wg.Wait()
	p.pool.Close()
	p.logger.Println("Event processor stopped")
}

// worker processes events in a loop
func (p *EventProcessor) worker(id int) {
	defer p.wg.Done()

	p.logger.Printf("Worker %d started", id)

	for {
		select {
		case <-p.stopCh:
			p.logger.Printf("Worker %d stopped", id)
			return
		default:
			// Get an event from the pool
			event := p.pool.GetEvent()

			// Process the event
			p.processEvent(event)

			// Recycle the event
			p.pool.RecycleEvent(event)
		}
	}
}

// processEvent handles a single event
func (p *EventProcessor) processEvent(event *eventpool.Event) {
	start := time.Now()

	// Map event type to domain event type
	var eventType domain.EventType
	switch event.Type {
	case eventpool.TypeDelivered:
		eventType = domain.EventDelivered
		p.metricsCollector.IncrementEventCount("delivered")
	case eventpool.TypeBounced:
		eventType = domain.EventBounced
		p.metricsCollector.IncrementEventCount("bounced")
	default:
		p.logger.Printf("Unknown event type: %s", event.Type)
		return
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Record the event
	if err := p.domainService.RecordEvent(ctx, event.Domain, eventType); err != nil {
		p.logger.Printf("Error recording event for domain %s: %v", event.Domain, err)
	}

	// Track metrics
	duration := time.Since(start)
	p.metricsCollector.RecordEventProcessingTime(duration)
}

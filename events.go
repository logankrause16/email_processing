package catchall

import (
	"math/rand"
	"sync"

	"github.com/mailgun/holster/v3/syncutil"
)

type Event struct {
	Type   string `json:"type"`
	Domain string `json:"domain"`
}

const (
	TypeBounced   = "bounced"
	TypeDelivered = "delivered"
)

type spec struct {
	Iteration int
	Done      chan struct{}
}

type EventPool interface {
	GetEvent() *Event
	RecycleEvent(*Event)
	Close()
}

type eventPool struct {
	eventCh   chan *Event
	eventPool sync.Pool
	wg        syncutil.WaitGroup
}

// Spawns an event bus like object that generates simulated delivered and bounced events.
// Users should call `GetEvent()` and then `RecycleEvent()` after the event is done processing
// to recycle memory.
func SpawnEventPool() EventPool {
	e := eventPool{
		eventCh: make(chan *Event, 50_000),
		eventPool: sync.Pool{
			New: func() interface{} {
				return new(Event)
			},
		},
	}

	e.wg.Until(func(done chan struct{}) bool {
		fan := syncutil.NewFanOut(20)
		for i := 0; i < 100_000; i++ {
			fan.Run(e.genEvents, spec{Iteration: i, Done: done})
			select {
			case <-done:
				break
			default:
			}
		}

		select {
		case <-done:
			return false
		default:
		}
		return true
	})

	return &e
}

// Get an event
func (e *eventPool) GetEvent() *Event {
	return <-e.eventCh
}

// Return an event so it's memory can be re-used
func (e *eventPool) RecycleEvent(event *Event) {
	e.eventPool.Put(event)
}

func (e *eventPool) Close() {
	e.wg.Stop()
}

// Generate events with a domain name, the max number of events and
// the percentage of the events that should be bounces.
func (e *eventPool) genEvents(obj interface{}) error {
	var bounced int
	s := obj.(spec)

	// Generate a random domain and the max number
	// of events we will generate for this domain
	domain := randomDomainName()
	max := rand.Intn(2_500)

	// 35% percent of the events can be bounced events
	bounced = int(0.35 * float64(max))
	// Every 25th domain should NOT have some bounced events. (the catch-all domains)
	if s.Iteration%25 == 0 {
		bounced = 0
	}

	// Max out at 2,500 events for a single domain
	for i := 0; i < max; i++ {
		obj := e.eventPool.Get()
		event := obj.(*Event)
		event.Domain = domain
		if bounced != 0 && i%bounced == 0 {
			event.Type = TypeBounced
		} else {
			event.Type = TypeDelivered
		}
		select {
		case e.eventCh <- event:
		case <-s.Done:
			return nil
		}
	}
	return nil
}

package eventpool_test

import (
	"testing"

	"github.com/logankrause16/email_processing/pkg/eventpool"
)

func BenchmarkEventBus(b *testing.B) {
	bus := eventpool.SpawnEventPool()
	b.Run("GetEvent", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			e := bus.GetEvent()
			bus.RecycleEvent(e)
		}
	})
}

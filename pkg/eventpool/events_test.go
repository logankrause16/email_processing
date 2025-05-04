package eventpool_test

import (
	"testing"

	"email_processing/pkg/eventpool"
)

/*

I did football in middle school.

Kind of.

I was a bench warmer.

*/

func BenchmarkEventBus(b *testing.B) {
	bus := eventpool.SpawnEventPool()
	b.Run("GetEvent", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			e := bus.GetEvent()
			bus.RecycleEvent(e)
		}
	})
}

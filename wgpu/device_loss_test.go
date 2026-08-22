package wgpu

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestDeviceLostCallbackDeliversTypedReasonAndMessageOnce(t *testing.T) {
	var calls atomic.Int32
	var gotReason DeviceLostReason
	var gotMessage string
	id := registerDeviceLostCallback(func(reason DeviceLostReason, message string) {
		calls.Add(1)
		gotReason, gotMessage = reason, message
	})

	message := "adapter reset"
	view := stringToStringView(message)
	handleDeviceLostCallback(uintptr(DeviceLostReasonUnknown), view, id)
	handleDeviceLostCallback(uintptr(DeviceLostReasonDestroyed), view, id)

	if calls.Load() != 1 || gotReason != DeviceLostReasonUnknown || gotMessage != message {
		t.Fatalf("callback calls=%d reason=%v message=%q", calls.Load(), gotReason, gotMessage)
	}
}

func TestDeviceLostCallbackReleaseRaceHasAtMostOneWinner(t *testing.T) {
	for iteration := 0; iteration < 100; iteration++ {
		var calls atomic.Int32
		id := registerDeviceLostCallback(func(DeviceLostReason, string) { calls.Add(1) })
		start := make(chan struct{})
		var group sync.WaitGroup
		group.Add(2)
		go func() { defer group.Done(); <-start; unregisterDeviceLostCallback(id) }()
		go func() {
			defer group.Done()
			<-start
			handleDeviceLostCallback(uintptr(DeviceLostReasonUnknown), StringView{}, id)
		}()
		close(start)
		group.Wait()
		if calls.Load() > 1 {
			t.Fatalf("iteration %d delivered %d callbacks", iteration, calls.Load())
		}
		if callback := takeDeviceLostCallback(id); callback != nil {
			t.Fatalf("iteration %d leaked callback registration", iteration)
		}
	}
}

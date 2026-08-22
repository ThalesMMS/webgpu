package wgpu

import (
	"sync"
	"sync/atomic"
	"testing"
)

type deviceDestroyProcStub struct {
	calls  atomic.Int32
	handle uintptr
	onCall func()
}

func (p *deviceDestroyProcStub) Call(args ...uintptr) (uintptr, uintptr, error) {
	p.calls.Add(1)
	if len(args) == 1 {
		p.handle = args[0]
	}
	if p.onCall != nil {
		p.onCall()
	}
	return 0, 0, nil
}

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

func TestCompleteDeviceDestroyedDeliversStableReasonOnce(t *testing.T) {
	var calls atomic.Int32
	var gotReason DeviceLostReason
	var gotMessage string
	id := registerDeviceLostCallback(func(reason DeviceLostReason, message string) {
		calls.Add(1)
		gotReason, gotMessage = reason, message
	})
	d := &Device{deviceLostCallbackID: id}

	completeDeviceDestroyed(d)
	completeDeviceDestroyed(d)

	if calls.Load() != 1 || gotReason != DeviceLostReasonDestroyed || gotMessage != "device destroyed" {
		t.Fatalf("callback calls=%d reason=%v message=%q", calls.Load(), gotReason, gotMessage)
	}
	if d.deviceLostCallbackID != 0 {
		t.Fatalf("device loss callback id = %d, want 0", d.deviceLostCallbackID)
	}
	if callback := takeDeviceLostCallback(id); callback != nil {
		t.Fatal("device loss callback registration leaked")
	}
}

func TestDeviceDestroyCallsNativeBeforeFallbackAndIsIdempotent(t *testing.T) {
	stub := &deviceDestroyProcStub{}
	original := procDeviceDestroy
	procDeviceDestroy = stub
	t.Cleanup(func() { procDeviceDestroy = original })

	var calls atomic.Int32
	id := registerDeviceLostCallback(func(reason DeviceLostReason, message string) {
		if stub.calls.Load() != 1 {
			t.Fatalf("native destroy calls = %d when fallback ran, want 1", stub.calls.Load())
		}
		if reason != DeviceLostReasonDestroyed || message != "device destroyed" {
			t.Fatalf("fallback reason=%v message=%q", reason, message)
		}
		calls.Add(1)
	})
	d := &Device{handle: 77, deviceLostCallbackID: id}

	d.Destroy()
	d.Destroy()

	if stub.calls.Load() != 1 || stub.handle != d.handle {
		t.Fatalf("native destroy calls=%d handle=%d, want 1 and %d", stub.calls.Load(), stub.handle, d.handle)
	}
	if calls.Load() != 1 || d.handle != 77 || d.deviceLostCallbackID != 0 {
		t.Fatalf("fallback calls=%d handle=%d callback id=%d", calls.Load(), d.handle, d.deviceLostCallbackID)
	}
	if callback := takeDeviceLostCallback(id); callback != nil {
		t.Fatal("device loss callback registration leaked")
	}
}

func TestDeviceDestroyIgnoresNilAndReleasedDevice(t *testing.T) {
	stub := &deviceDestroyProcStub{}
	original := procDeviceDestroy
	procDeviceDestroy = stub
	t.Cleanup(func() { procDeviceDestroy = original })

	var nilDevice *Device
	nilDevice.Destroy()
	(&Device{}).Destroy()

	if stub.calls.Load() != 0 {
		t.Fatalf("native destroy calls=%d, want 0", stub.calls.Load())
	}
}

func TestCompleteDeviceDestroyedAndNativeCallbackHaveOneWinner(t *testing.T) {
	for iteration := 0; iteration < 100; iteration++ {
		var calls atomic.Int32
		id := registerDeviceLostCallback(func(DeviceLostReason, string) { calls.Add(1) })
		d := &Device{deviceLostCallbackID: id}
		start := make(chan struct{})
		var group sync.WaitGroup
		group.Add(2)
		go func() { defer group.Done(); <-start; completeDeviceDestroyed(d) }()
		go func() {
			defer group.Done()
			<-start
			handleDeviceLostCallback(uintptr(DeviceLostReasonUnknown), StringView{}, id)
		}()
		close(start)
		group.Wait()

		if calls.Load() != 1 {
			t.Fatalf("iteration %d delivered %d callbacks", iteration, calls.Load())
		}
		if d.deviceLostCallbackID != 0 {
			t.Fatalf("iteration %d callback id=%d, want 0", iteration, d.deviceLostCallbackID)
		}
		if callback := takeDeviceLostCallback(id); callback != nil {
			t.Fatalf("iteration %d leaked callback registration", iteration)
		}
	}
}

func TestDeviceDestroyAndNativeCallbackHaveOneWinner(t *testing.T) {
	enteredNativeDestroy := make(chan struct{})
	allowFallback := make(chan struct{})
	stub := &deviceDestroyProcStub{
		onCall: func() {
			close(enteredNativeDestroy)
			<-allowFallback
		},
	}
	original := procDeviceDestroy
	procDeviceDestroy = stub
	t.Cleanup(func() { procDeviceDestroy = original })

	var calls atomic.Int32
	id := registerDeviceLostCallback(func(DeviceLostReason, string) { calls.Add(1) })
	d := &Device{handle: 91, deviceLostCallbackID: id}
	destroyDone := make(chan struct{})
	nativeDone := make(chan struct{})
	go func() { defer close(destroyDone); d.Destroy() }()
	<-enteredNativeDestroy
	go func() {
		defer close(nativeDone)
		handleDeviceLostCallback(uintptr(DeviceLostReasonUnknown), StringView{}, id)
	}()
	close(allowFallback)
	<-destroyDone
	<-nativeDone

	if stub.calls.Load() != 1 || calls.Load() != 1 {
		t.Fatalf("native destroy calls=%d callback calls=%d, want 1 each", stub.calls.Load(), calls.Load())
	}
	if d.deviceLostCallbackID != 0 {
		t.Fatalf("device loss callback id = %d, want 0", d.deviceLostCallbackID)
	}
	if callback := takeDeviceLostCallback(id); callback != nil {
		t.Fatal("device loss callback registration leaked")
	}
}

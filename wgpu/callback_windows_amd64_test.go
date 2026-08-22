//go:build windows && amd64

package wgpu

import (
	"testing"
	"unsafe"
)

func TestDeviceLostCallbackEntryWindowsAMD64ABI(t *testing.T) {
	message := "indirect device loss"
	view := stringToStringView(message)
	var got string
	id := registerDeviceLostCallback(func(_ DeviceLostReason, value string) { got = value })
	deviceLostCallbackEntry(
		uintptr(DeviceLostReasonUnknown), uintptr(unsafe.Pointer(&view)), id, 0,
	)
	if got != message {
		t.Fatalf("message = %q, want %q", got, message)
	}
}

func TestABICallbackStringViewWindowsAMD64(t *testing.T) {
	if got := callbackStringView(0); got != (StringView{}) {
		t.Fatalf("callbackStringView(0) = %#v, want empty", got)
	}

	message := []byte("callback message")
	want := StringView{
		Data:   uintptr(unsafe.Pointer(&message[0])),
		Length: uintptr(len(message)),
	}
	if got := callbackStringView(uintptr(unsafe.Pointer(&want))); got != want {
		t.Fatalf("callbackStringView(valid) = %#v, want %#v", got, want)
	}
}

func TestABIAdapterCallbackEntryWindowsAMD64(t *testing.T) {
	const requestID = uintptr(201)
	req := registerTestAdapterRequest(t, requestID)
	message := []byte("callback message")
	view := StringView{
		Data:   uintptr(unsafe.Pointer(&message[0])),
		Length: uintptr(len(message)),
	}

	adapterCallbackEntry(7, 0, uintptr(unsafe.Pointer(&view)), requestID, 0)

	assertCallbackCompleted(t, req.done, req.message)
	if req.status != RequestAdapterStatus(7) {
		t.Fatalf("status = %d, want 7", req.status)
	}
}

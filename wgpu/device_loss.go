package wgpu

import (
	"sync"

	"github.com/go-webgpu/goffi/ffi"
)

// DeviceLostCallback receives one native device-loss notification.
type DeviceLostCallback func(reason DeviceLostReason, message string)

var (
	deviceLostCallbacks    = make(map[uintptr]DeviceLostCallback)
	deviceLostCallbacksMu  sync.Mutex
	deviceLostCallbackID   uintptr
	deviceLostCallbackPtr  uintptr
	deviceLostCallbackOnce sync.Once
)

func registerDeviceLostCallback(callback DeviceLostCallback) uintptr {
	if callback == nil {
		return 0
	}
	deviceLostCallbacksMu.Lock()
	defer deviceLostCallbacksMu.Unlock()
	deviceLostCallbackID++
	if deviceLostCallbackID == 0 {
		deviceLostCallbackID++
	}
	deviceLostCallbacks[deviceLostCallbackID] = callback
	return deviceLostCallbackID
}

func takeDeviceLostCallback(id uintptr) DeviceLostCallback {
	if id == 0 {
		return nil
	}
	deviceLostCallbacksMu.Lock()
	callback := deviceLostCallbacks[id]
	delete(deviceLostCallbacks, id)
	deviceLostCallbacksMu.Unlock()
	return callback
}

func unregisterDeviceLostCallback(id uintptr) { _ = takeDeviceLostCallback(id) }

func handleDeviceLostCallback(reason uintptr, message StringView, id uintptr) uintptr {
	callback := takeDeviceLostCallback(id)
	if callback != nil {
		callback(DeviceLostReason(reason), stringViewToString(message))
	}
	return 0
}

func initDeviceLostCallback() {
	deviceLostCallbackPtr = ffi.NewCallback(deviceLostCallbackEntry)
}

package wgpu

import (
	"testing"
	"unsafe"
)

func TestStringViewToStringCopiesNativeBacking(t *testing.T) {
	backing := []byte("Apple M2")
	want := string(backing)
	view := StringView{
		Data:   uintptr(unsafe.Pointer(&backing[0])),
		Length: uintptr(len(backing)),
	}

	got := stringViewToString(view)
	for index := range backing {
		backing[index] = 0
	}

	if got != want {
		t.Fatalf("stringViewToString = %q after native backing changed, want %q", got, want)
	}
}

package scanner

import "sync/atomic"

// Diagnostics: when a recorder is set, every answer heard on the network
// (Bonjour, NetBIOS, SSDP/UPnP, Art-Net, sACN, PTP, PJLink) is passed to
// it, so a "diagnostic bundle" can show how real gear answers.

type diagFunc func(kind, from string, data []byte)

var diagSink atomic.Value // diagFunc

// SetDiag sets (or with nil removes) the recorder.
func SetDiag(f func(kind, from string, data []byte)) {
	diagSink.Store(diagFunc(f))
}

func diag(kind, from string, data []byte) {
	if f, _ := diagSink.Load().(diagFunc); f != nil {
		f(kind, from, append([]byte(nil), data...))
	}
}

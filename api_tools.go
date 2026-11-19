package main

import (
	"bufio"
	"context"
	"math"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wasabifilm/wasaping/scanner"
)

// Network tools for the detail panel: ping with link quality, traceroute.

var reHostname = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)

// toolTarget accepts an IPv4 address or a host name (never something that
// could be read as a command-line option).
func toolTarget(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if ip := net.ParseIP(s).To4(); ip != nil {
		return ip.String(), true
	}
	return s, reHostname.MatchString(s)
}

// handlePingTest sends `count` pings and reports each one, then loss,
// min/avg/max and jitter: the quality of the link to that device.
func handlePingTest(w http.ResponseWriter, r *http.Request) {
	host, ok := toolTarget(r.URL.Query().Get("host"))
	if !ok {
		http.Error(w, "indirizzo non valido", http.StatusBadRequest)
		return
	}
	count, _ := strconv.Atoi(r.URL.Query().Get("count"))
	if count <= 0 || count > 200 {
		count = 20
	}
	send, ok := sseStart(w)
	if !ok {
		return
	}
	activeScan.Add(1)
	defer activeScan.Add(-1)
	var rtts []float64
	for i := 1; i <= count && r.Context().Err() == nil; i++ {
		start := time.Now()
		alive, rtt := scanner.SystemPing(r.Context(), host, time.Second)
		ms := float64(rtt.Microseconds()) / 1000
		if alive {
			rtts = append(rtts, ms)
		}
		send("ping", map[string]any{"seq": i, "ok": alive, "ms": ms})
		if d := 300*time.Millisecond - time.Since(start); d > 0 && i < count {
			select {
			case <-time.After(d):
			case <-r.Context().Done():
			}
		}
	}
	st := map[string]any{"sent": count, "received": len(rtts), "loss": 100 * float64(count-len(rtts)) / float64(count)}
	if len(rtts) > 0 {
		lo, hi, sum := rtts[0], rtts[0], 0.0
		for _, v := range rtts {
			lo, hi, sum = math.Min(lo, v), math.Max(hi, v), sum+v
		}
		jit := 0.0
		for i := 1; i < len(rtts); i++ {
			jit += math.Abs(rtts[i] - rtts[i-1])
		}
		if len(rtts) > 1 {
			jit /= float64(len(rtts) - 1)
		}
		st["min"], st["avg"], st["max"], st["jitter"] = lo, sum/float64(len(rtts)), hi, jit
	}
	send("done", st)
}

// handleTrace streams the system traceroute, line by line.
func handleTrace(w http.ResponseWriter, r *http.Request) {
	host, ok := toolTarget(r.URL.Query().Get("host"))
	if !ok {
		http.Error(w, "indirizzo non valido", http.StatusBadRequest)
		return
	}
	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "tracert", "-d", "-h", "20", "-w", "1000", host)
	default:
		if _, err := exec.LookPath("traceroute"); err == nil {
			cmd = exec.CommandContext(ctx, "traceroute", "-n", "-q", "1", "-w", "1", "-m", "20", host)
		} else {
			cmd = exec.CommandContext(ctx, "tracepath", "-n", host)
		}
	}
	scanner.HideWindow(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cmd.Stderr = cmd.Stdout
	send, ok := sseStart(w)
	if !ok {
		return
	}
	if err := cmd.Start(); err != nil {
		send("line", "Traceroute non disponibile: "+err.Error())
		send("done", nil)
		return
	}
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		if line := strings.TrimRight(sc.Text(), " \r"); line != "" {
			send("line", line)
		}
	}
	cmd.Wait()
	send("done", nil)
}

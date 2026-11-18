package scanner

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DefaultProbePorts are tried to detect if a host is up without raw sockets.
// A "connection refused" answer also proves the host is alive.
var DefaultProbePorts = []int{80, 443, 22, 445, 139, 8080, 62078, 5000, 3389, 554}

// CommonPorts is the default list checked on hosts found alive.
var CommonPorts = []int{21, 22, 23, 53, 80, 139, 443, 445, 548, 554, 631, 1883, 3389, 5000, 5900, 8000, 8080, 8443, 9100, 32400, 62078}

// tcpProbe tries the ports in parallel; it returns true at the first proof of life.
func tcpProbe(ctx context.Context, ip string, ports []int, timeout time.Duration) (bool, time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type res struct {
		alive bool
		rtt   time.Duration
	}
	ch := make(chan res, len(ports))
	d := net.Dialer{}
	for _, p := range ports {
		go func(p int) {
			start := time.Now()
			c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(p)))
			if err == nil {
				c.Close()
				ch <- res{true, time.Since(start)}
				return
			}
			ch <- res{isRefused(err), time.Since(start)}
		}(p)
	}
	for range ports {
		r := <-ch
		if r.alive {
			return true, r.rtt
		}
	}
	return false, 0
}

func isRefused(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "refused") || strings.Contains(s, "actively refused")
}

// portOpen reports whether a TCP port accepts connections.
func portOpen(ctx context.Context, ip string, port int, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// systemPing uses the OS ping command (ICMP without needing admin rights).
func systemPing(ctx context.Context, ip string, timeout time.Duration) bool {
	secs := int(timeout.Round(time.Second) / time.Second)
	if secs < 1 {
		secs = 1
	}
	var args []string
	switch runtime.GOOS {
	case "windows":
		args = []string{"-n", "1", "-w", strconv.Itoa(int(timeout / time.Millisecond)), ip}
	case "darwin", "freebsd", "openbsd", "netbsd":
		args = []string{"-c", "1", "-t", strconv.Itoa(secs), ip}
	default:
		args = []string{"-c", "1", "-W", strconv.Itoa(secs), ip}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ping", args...)
	hideWindow(cmd)
	out, err := cmd.Output()
	if runtime.GOOS == "windows" {
		// Windows can exit 0 on "destination unreachable": require a real reply.
		return strings.Contains(strings.ToUpper(string(out)), "TTL=")
	}
	return err == nil
}

// reverseDNS looks up a hostname for ip with a short timeout.
func reverseDNS(ctx context.Context, ip string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

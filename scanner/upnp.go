package scanner

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// upnpInfo is what a device says in its UPnP description (TVs, routers,
// NAS, media players, many audio/video devices).
type upnpInfo struct {
	friendly, maker, model, number, devType string
}

// upnpDiscover sends an SSDP M-SEARCH and reads the description XML of every
// device that answers (only from the device's own address).
func upnpDiscover(ctx context.Context, wait time.Duration) map[string]*upnpInfo {
	out := map[string]*upnpInfo{}
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return out
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	msg := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 1\r\nST: upnp:rootdevice\r\nUSER-AGENT: wasaping UPnP/1.1\r\n\r\n"
	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	c.WriteToUDP([]byte(msg), dst)
	c.WriteToUDP([]byte(msg), dst)

	locations := map[string]string{} // ip -> description URL
	readUntil(ctx, c, func(b []byte, from *net.UDPAddr) {
		resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(b)), nil)
		if err != nil {
			return
		}
		resp.Body.Close()
		loc := resp.Header.Get("Location")
		u, err := url.Parse(loc)
		ip := from.IP.String()
		if err != nil || u.Hostname() != ip || (u.Scheme != "http" && u.Scheme != "https") {
			return // only descriptions served by the device itself
		}
		if _, seen := locations[ip]; !seen {
			locations[ip] = loc
		}
	})

	var mu sync.Mutex
	var wg sync.WaitGroup
	client := deviceHTTPClient(2 * time.Second)
	for ip, loc := range locations {
		wg.Add(1)
		go func(ip, loc string) {
			defer wg.Done()
			if info := fetchUPnP(context.Background(), client, loc); info != nil {
				mu.Lock()
				out[ip] = info
				mu.Unlock()
			}
		}(ip, loc)
	}
	wg.Wait()
	return out
}

func fetchUPnP(ctx context.Context, client *http.Client, loc string) *upnpInfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var d struct {
		Device struct {
			DeviceType   string `xml:"deviceType"`
			FriendlyName string `xml:"friendlyName"`
			Manufacturer string `xml:"manufacturer"`
			ModelName    string `xml:"modelName"`
			ModelNumber  string `xml:"modelNumber"`
		} `xml:"device"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&d); err != nil {
		return nil
	}
	t := d.Device.DeviceType // urn:schemas-upnp-org:device:MediaRenderer:1
	if p := strings.Split(t, ":"); len(p) >= 4 {
		t = p[3]
	}
	return &upnpInfo{
		friendly: strings.TrimSpace(d.Device.FriendlyName),
		maker:    strings.TrimSpace(d.Device.Manufacturer),
		model:    strings.TrimSpace(d.Device.ModelName),
		number:   strings.TrimSpace(d.Device.ModelNumber),
		devType:  t,
	}
}

// deviceHTTPClient talks to devices on the LAN: short timeouts, self-signed
// certificates accepted (we only read a title or a description), redirects
// only to the same device.
func deviceHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			ResponseHeaderTimeout: timeout,
			DisableKeepAlives:     true,
			Proxy:                 nil,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Hostname() != via[0].URL.Hostname() {
				return errors.New("redirect elsewhere")
			}
			return nil
		},
	}
}

var reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// webTitle reads the <title> of a device's web page (router, switch,
// console, NAS panel…), trying the given web ports in order.
func webTitle(ctx context.Context, ip string, ports []int) string {
	client := deviceHTTPClient(1500 * time.Millisecond)
	for _, p := range ports {
		scheme := "http"
		if p == 443 || p == 8443 {
			scheme = "https"
		}
		u := scheme + "://" + net.JoinHostPort(ip, itoa(p)) + "/"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "wasaping")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
		resp.Body.Close()
		if m := reTitle.FindSubmatch(b); m != nil {
			t := strings.Join(strings.Fields(html.UnescapeString(string(m[1]))), " ")
			if t != "" && len(t) < 200 && !boringTitle(t) {
				return t
			}
		}
	}
	return ""
}

func itoa(n int) string {
	var b [8]byte
	i := len(b)
	for {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			return string(b[i:])
		}
	}
}

// boringTitle recognizes placeholder page titles that tell nothing.
func boringTitle(t string) bool {
	l := strings.ToLower(strings.Trim(t, " .…"))
	for _, w := range []string{"404", "not found", "opening", "loading", "redirect", "error", "untitled", "document", "index", "welcome", "login", "please wait", "403", "forbidden", "unauthorized"} {
		if l == w || strings.HasPrefix(l, w) || strings.Contains(l, " "+w) && len(l) < 30 {
			return true
		}
	}
	return false
}

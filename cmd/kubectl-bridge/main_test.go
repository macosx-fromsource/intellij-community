package main

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// The warning exists to catch an accidental exposure of an unauthenticated
// server, so an unspecified address must count as non-loopback while names that
// resolve to loopback must not cry wolf.
func TestIsLoopback(t *testing.T) {
	for _, tc := range []struct {
		address string
		want    bool
	}{
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"::1", true},
		{localhostName, true},
		{"0.0.0.0", false},
		{"::", false},
		{"", false},
		{"10.10.1.248", false},
		{"not-a-real-host.invalid", false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			require.Equal(t, tc.want, isLoopback(tc.address))
		})
	}
}

// The printed URL has to be one the browser can actually open: a name for the
// everywhere/loopback cases, and the literal address otherwise so an IPv6 bind
// is not silently swapped for a name that resolves to IPv4.
func TestBrowserHost(t *testing.T) {
	for _, tc := range []struct {
		address string
		want    string
	}{
		{"127.0.0.1", localhostName},
		{"0.0.0.0", localhostName},
		{"::", localhostName},
		{"", localhostName},
		{"::1", "::1"},
		{"10.10.1.248", "10.10.1.248"},
	} {
		t.Run(tc.address, func(t *testing.T) {
			require.Equal(t, tc.want, browserHost(tc.address))
		})
	}
}

// An IPv6 literal must bind. Naive "host:port" concatenation produces
// "::1:8090", which fails with "too many colons in address".
func TestListenIPv6Loopback(t *testing.T) {
	ln, url, err := listen("::1", 0)
	if err != nil {
		t.Skipf("no IPv6 loopback available: %v", err)
	}

	t.Cleanup(func() { _ = ln.Close() })

	port := ln.Addr().(*net.TCPAddr).Port
	require.Equal(t, "http://[::1]:"+strconv.Itoa(port), url)
}

// The default bind still produces the friendly localhost URL on the port that
// was actually taken.
func TestListenIPv4Loopback(t *testing.T) {
	ln, url, err := listen("127.0.0.1", 0)
	require.NoError(t, err)

	t.Cleanup(func() { _ = ln.Close() })

	port := ln.Addr().(*net.TCPAddr).Port
	require.Equal(t, "http://"+localhostName+":"+strconv.Itoa(port), url)
}

// When the requested port is taken, listen must fall back to a free one rather
// than fail, and report the port it actually got.
func TestListenFallsBackToFreePort(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = busy.Close() })

	busyPort := busy.Addr().(*net.TCPAddr).Port

	ln, url, err := listen("127.0.0.1", busyPort)
	require.NoError(t, err)

	t.Cleanup(func() { _ = ln.Close() })

	require.NotEqual(t, busyPort, ln.Addr().(*net.TCPAddr).Port)
	require.Equal(t, "http://"+localhostName+":"+strconv.Itoa(ln.Addr().(*net.TCPAddr).Port), url)
}

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunQueriesLocalServerAndWritesCSV(t *testing.T) {
	address := startLocalNTPServer(t)
	csvPath := filepath.Join(t.TempDir(), "samples.csv")
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{
		"--server", address,
		"--timeout", "1s",
		"--csv", csvPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SERVER") || !strings.Contains(stdout.String(), address) {
		t.Fatalf("table output %q does not contain the header and configured server", stdout.String())
	}

	csvData, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("read CSV output: %v", err)
	}
	if !strings.Contains(string(csvData), "server,offset,round_trip,measured_at\n") || !strings.Contains(string(csvData), address) {
		t.Fatalf("CSV output %q does not contain its header and sample", csvData)
	}
}

func TestRunRejectsNonpositiveTimeout(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"--timeout", "0s"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "timeout must be positive") {
		t.Fatalf("stderr = %q, want timeout validation message", stderr.String())
	}
}

func startLocalNTPServer(t *testing.T) string {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen for local NTP fixture: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		request := make([]byte, 512)
		n, peer, err := conn.ReadFromUDP(request)
		if err != nil || n < 48 {
			return
		}
		response := make([]byte, 48)
		response[0] = 4<<3 | 4
		response[1] = 2
		copy(response[24:32], request[40:48])
		putNTPTime(response[32:40], time.Now())
		putNTPTime(response[40:48], time.Now().Add(time.Millisecond))
		_, _ = conn.WriteToUDP(response, peer)
	}()
	return conn.LocalAddr().String()
}

func putNTPTime(dst []byte, value time.Time) {
	const epochOffset = 2_208_988_800
	seconds := uint32(value.Unix() + epochOffset)
	fraction := uint32((uint64(value.Nanosecond()) << 32) / 1_000_000_000)
	binary.BigEndian.PutUint32(dst[:4], seconds)
	binary.BigEndian.PutUint32(dst[4:8], fraction)
}

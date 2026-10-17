package ntpclient

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestProbeCalculatesOffsetFromServerTimestamps(t *testing.T) {
	const serverOffset = 80 * time.Millisecond
	address := startUDPFixture(t, func(request []byte) []byte {
		if len(request) != 48 {
			t.Errorf("request length = %d, want 48", len(request))
		}

		response := make([]byte, 48)
		response[0] = 4<<3 | 4
		response[1] = 2
		copy(response[24:32], request[40:48])
		serverReceive := time.Now().Add(serverOffset)
		serverTransmit := serverReceive.Add(2 * time.Millisecond)
		putTimestamp(response[32:40], serverReceive)
		putTimestamp(response[40:48], serverTransmit)
		return response
	})

	sample, err := Probe(context.Background(), address, time.Second)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if delta := sample.Offset - serverOffset; delta < -25*time.Millisecond || delta > 25*time.Millisecond {
		t.Fatalf("offset = %s, want near %s", sample.Offset, serverOffset)
	}
	if sample.RoundTrip < 0 || sample.RoundTrip > time.Second {
		t.Fatalf("round trip = %s, want a non-negative value under one second", sample.RoundTrip)
	}
	if sample.Server != address {
		t.Fatalf("server = %q, want %q", sample.Server, address)
	}
	if sample.MeasuredAt.IsZero() {
		t.Fatal("measured-at timestamp is zero")
	}
}

func TestProbeRejectsShortResponse(t *testing.T) {
	address := startUDPFixture(t, func([]byte) []byte {
		return make([]byte, 12)
	})

	_, err := Probe(context.Background(), address, time.Second)
	if err == nil {
		t.Fatal("Probe() error = nil, want short-packet error")
	}
}

func TestProbeRejectsInvalidMode(t *testing.T) {
	address := startUDPFixture(t, func(request []byte) []byte {
		response := validResponse(request)
		response[0] = 4<<3 | 3
		return response
	})

	_, err := Probe(context.Background(), address, time.Second)
	if err == nil {
		t.Fatal("Probe() error = nil, want invalid-mode error")
	}
}

func TestProbeRejectsInvalidVersion(t *testing.T) {
	address := startUDPFixture(t, func(request []byte) []byte {
		response := validResponse(request)
		response[0] = 2<<3 | 4
		return response
	})

	_, err := Probe(context.Background(), address, time.Second)
	if err == nil {
		t.Fatal("Probe() error = nil, want invalid-version error")
	}
}

func TestProbeRejectsUnsynchronizedStratum(t *testing.T) {
	address := startUDPFixture(t, func(request []byte) []byte {
		response := validResponse(request)
		response[1] = 16
		return response
	})

	_, err := Probe(context.Background(), address, time.Second)
	if err == nil {
		t.Fatal("Probe() error = nil, want unsynchronized-stratum error")
	}
}

func TestProbeTimesOutWithinConfiguredDuration(t *testing.T) {
	address := startUDPFixture(t, func([]byte) []byte { return nil })
	timeout := 80 * time.Millisecond
	start := time.Now()

	_, err := Probe(context.Background(), address, timeout)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Probe() error = nil, want timeout error")
	}
	if elapsed < timeout/2 || elapsed > 500*time.Millisecond {
		t.Fatalf("elapsed = %s, want roughly %s and under 500ms", elapsed, timeout)
	}
}

func startUDPFixture(t *testing.T, reply func([]byte) []byte) string {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen for local UDP fixture: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		buffer := make([]byte, 512)
		n, peer, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		response := reply(append([]byte(nil), buffer[:n]...))
		if response != nil {
			_, _ = conn.WriteToUDP(response, peer)
		}
	}()

	return conn.LocalAddr().String()
}

func validResponse(request []byte) []byte {
	response := make([]byte, 48)
	response[0] = 4<<3 | 4
	response[1] = 2
	copy(response[24:32], request[40:48])
	putTimestamp(response[32:40], time.Now())
	putTimestamp(response[40:48], time.Now().Add(time.Millisecond))
	return response
}

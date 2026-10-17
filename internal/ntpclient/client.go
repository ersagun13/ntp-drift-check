package ntpclient

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
)

const (
	packetSize    = 48
	ntpUnixOffset = 2_208_988_800
	eraSeconds    = int64(1) << 32
)

// Sample is one NTP exchange measured by Probe.
type Sample struct {
	Server     string
	Offset     time.Duration
	RoundTrip  time.Duration
	MeasuredAt time.Time
}

// Probe sends one NTP client request and estimates the server's clock offset.
// It only reads the clock; it never changes the local system time.
func Probe(ctx context.Context, address string, timeout time.Duration) (Sample, error) {
	if ctx == nil {
		return Sample{}, fmt.Errorf("context must not be nil")
	}
	if address == "" {
		return Sample{}, fmt.Errorf("server address must not be empty")
	}
	if timeout <= 0 {
		return Sample{}, fmt.Errorf("timeout must be positive")
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(probeCtx, "udp", address)
	if err != nil {
		return Sample{}, fmt.Errorf("connect to NTP server %q: %w", address, err)
	}
	defer conn.Close()

	deadline, ok := probeCtx.Deadline()
	if ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return Sample{}, fmt.Errorf("set NTP deadline: %w", err)
		}
	}
	stopDeadline := context.AfterFunc(probeCtx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	defer stopDeadline()

	t1 := time.Now()
	request := make([]byte, packetSize)
	request[0] = 0x23 // LI=0, NTPv4, client mode.
	putTimestamp(request[40:48], t1)
	if _, err := conn.Write(request); err != nil {
		return Sample{}, fmt.Errorf("send NTP request to %q: %w", address, err)
	}

	response := make([]byte, 512)
	n, err := conn.Read(response)
	t4 := time.Now()
	if err != nil {
		if probeCtx.Err() != nil {
			return Sample{}, fmt.Errorf("wait for NTP response from %q: %w", address, probeCtx.Err())
		}
		if networkErr, ok := err.(net.Error); ok && networkErr.Timeout() {
			return Sample{}, fmt.Errorf("wait for NTP response from %q: %w", address, context.DeadlineExceeded)
		}
		return Sample{}, fmt.Errorf("read NTP response from %q: %w", address, err)
	}
	if n < packetSize {
		return Sample{}, fmt.Errorf("NTP response from %q is %d bytes; need at least %d", address, n, packetSize)
	}
	response = response[:n]

	version := (response[0] >> 3) & 0x07
	mode := response[0] & 0x07
	leap := response[0] >> 6
	if version < 3 || version > 4 {
		return Sample{}, fmt.Errorf("NTP response from %q uses unsupported version %d", address, version)
	}
	if mode != 4 {
		return Sample{}, fmt.Errorf("NTP response from %q has mode %d; want server mode 4", address, mode)
	}
	if leap == 3 {
		return Sample{}, fmt.Errorf("NTP server %q reports that it is unsynchronized", address)
	}
	if response[1] == 0 || response[1] > 15 {
		return Sample{}, fmt.Errorf("NTP server %q returned unsynchronized stratum %d", address, response[1])
	}
	if string(response[24:32]) != string(request[40:48]) {
		return Sample{}, fmt.Errorf("NTP response from %q does not match the request timestamp", address)
	}
	if allZero(response[32:40]) || allZero(response[40:48]) {
		return Sample{}, fmt.Errorf("NTP response from %q has an empty server timestamp", address)
	}

	t2, err := decodeTimestamp(response[32:40], t4)
	if err != nil {
		return Sample{}, fmt.Errorf("decode server receive timestamp from %q: %w", address, err)
	}
	t3, err := decodeTimestamp(response[40:48], t4)
	if err != nil {
		return Sample{}, fmt.Errorf("decode server transmit timestamp from %q: %w", address, err)
	}

	offset := (t2.Sub(t1) + t3.Sub(t4)) / 2
	return Sample{
		Server:     address,
		Offset:     offset,
		RoundTrip:  t4.Sub(t1),
		MeasuredAt: t4.UTC(),
	}, nil
}

func putTimestamp(dst []byte, value time.Time) {
	seconds := uint32(value.Unix() + ntpUnixOffset)
	fraction := uint32((uint64(value.Nanosecond()) << 32) / 1_000_000_000)
	binary.BigEndian.PutUint32(dst[:4], seconds)
	binary.BigEndian.PutUint32(dst[4:8], fraction)
}

func decodeTimestamp(src []byte, near time.Time) (time.Time, error) {
	if len(src) < 8 {
		return time.Time{}, fmt.Errorf("timestamp is %d bytes; need 8", len(src))
	}
	seconds := int64(binary.BigEndian.Uint32(src[:4]))
	fraction := uint64(binary.BigEndian.Uint32(src[4:8]))
	currentNTPSec := near.Unix() + ntpUnixOffset
	era := int64(math.Round(float64(currentNTPSec-seconds) / float64(eraSeconds)))
	fullSeconds := seconds + era*eraSeconds
	nanoseconds := int64((fraction * 1_000_000_000) >> 32)
	return time.Unix(fullSeconds-ntpUnixOffset, nanoseconds).UTC(), nil
}

func allZero(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return true
}

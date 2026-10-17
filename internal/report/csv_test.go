package report

import (
	"bytes"
	"testing"
	"time"

	"github.com/ersagun13/ntp-drift-check/internal/ntpclient"
)

func TestWriteCSVWritesColumnsAndEscapesServerField(t *testing.T) {
	sample := ntpclient.Sample{
		Server:     `lab, "backup"`,
		Offset:     -1500 * time.Millisecond,
		RoundTrip:  12 * time.Millisecond,
		MeasuredAt: time.Date(2026, 10, 17, 20, 4, 5, 123456789, time.UTC),
	}
	var output bytes.Buffer

	if err := WriteCSV(&output, []ntpclient.Sample{sample}); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}

	want := "server,offset,round_trip,measured_at\n" +
		`"lab, ""backup""",-1.5s,12ms,2026-10-17T20:04:05.123456789Z` + "\n"
	if output.String() != want {
		t.Fatalf("CSV = %q, want %q", output.String(), want)
	}
}

func TestWriteCSVWritesHeaderForNoSamples(t *testing.T) {
	var output bytes.Buffer

	if err := WriteCSV(&output, nil); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}
	if got, want := output.String(), "server,offset,round_trip,measured_at\n"; got != want {
		t.Fatalf("CSV = %q, want %q", got, want)
	}
}

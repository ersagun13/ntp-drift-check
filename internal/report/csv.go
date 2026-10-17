package report

import (
	"encoding/csv"
	"io"

	"github.com/ersagun13/ntp-drift-check/internal/ntpclient"
)

// WriteCSV writes the measured samples with a fixed, machine-readable header.
func WriteCSV(output io.Writer, samples []ntpclient.Sample) error {
	writer := csv.NewWriter(output)
	if err := writer.Write([]string{"server", "offset", "round_trip", "measured_at"}); err != nil {
		return err
	}
	for _, sample := range samples {
		row := []string{
			sample.Server,
			sample.Offset.String(),
			sample.RoundTrip.String(),
			sample.MeasuredAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

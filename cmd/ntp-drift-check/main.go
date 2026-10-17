package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/ersagun13/ntp-drift-check/internal/ntpclient"
	"github.com/ersagun13/ntp-drift-check/internal/report"
)

const defaultServer = "time.google.com:123"

type stringList struct {
	values []string
}

func (list *stringList) String() string {
	return fmt.Sprint(list.values)
}

func (list *stringList) Set(value string) error {
	if value == "" {
		return fmt.Errorf("server address must not be empty")
	}
	list.values = append(list.values, value)
	return nil
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("ntp-drift-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: ntp-drift-check [--server host:port]... [--timeout duration] [--csv path]")
		fmt.Fprintln(stderr, "Query an NTP server and report estimated clock offset and round-trip time.")
		flags.PrintDefaults()
	}
	var servers stringList
	flags.Var(&servers, "server", "NTP server address; repeat to query more than one")
	timeout := flags.Duration("timeout", 2*time.Second, "maximum wait for each server")
	csvPath := flags.String("csv", "", "write successful samples to this CSV file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(flags.Args()) != 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %v\n", flags.Args())
		flags.Usage()
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(stderr, "timeout must be positive")
		return 2
	}
	if len(servers.values) == 0 {
		servers.values = []string{defaultServer}
	}

	samples := make([]ntpclient.Sample, 0, len(servers.values))
	failed := false
	for _, address := range servers.values {
		sample, err := ntpclient.Probe(ctx, address, *timeout)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", address, err)
			failed = true
			continue
		}
		samples = append(samples, sample)
	}
	if err := writeTable(stdout, samples); err != nil {
		fmt.Fprintf(stderr, "write report: %v\n", err)
		return 1
	}
	if *csvPath != "" {
		if err := writeCSVFile(*csvPath, samples); err != nil {
			fmt.Fprintf(stderr, "write CSV %q: %v\n", *csvPath, err)
			return 1
		}
	}
	if failed {
		return 1
	}
	return 0
}

func writeTable(output io.Writer, samples []ntpclient.Sample) error {
	writer := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "SERVER\tOFFSET\tROUND TRIP\tMEASURED AT"); err != nil {
		return err
	}
	for _, sample := range samples {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n",
			sample.Server,
			sample.Offset,
			sample.RoundTrip,
			sample.MeasuredAt.Format(time.RFC3339),
		); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func writeCSVFile(path string, samples []ntpclient.Sample) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writeErr := report.WriteCSV(file, samples)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

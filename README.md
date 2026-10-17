# ntp-drift-check

A small Go command-line tool that asks an NTP server for the time and prints an estimated clock offset and round-trip time. It reads the local clock but never changes it.

## Run it

Install Go 1.22 or newer. The default server is `time.google.com:123`:

```sh
go run ./cmd/ntp-drift-check
```

Choose servers, set a per-server timeout, or save successful samples as CSV:

```sh
go run ./cmd/ntp-drift-check \
  --server time.google.com:123 \
  --server time1.google.com:123 \
  --timeout 2s \
  --csv samples.csv
```

It needs outbound UDP access to the selected NTP servers. If a server times out or returns an invalid packet, the command reports the error and continues with the remaining servers; it exits nonzero if any query failed. CSV output contains successful samples, with a header even when there are none.

## Example output

Values below are illustrative; actual offset and latency depend on the network path.

```text
SERVER               OFFSET  ROUND TRIP  MEASURED AT
time.google.com:123  3.4ms   21.8ms      2026-10-17T12:34:56Z
```

The CSV columns are `server`, `offset`, `round_trip`, and `measured_at`.

## Notes

Offset is estimated from one NTP exchange. Unequal network delays in each direction affect that estimate, so this is useful for inspection and troubleshooting, not clock discipline or long-term drift measurement. Responses are not authenticated, so do not treat the result as a trusted time source. The default Google Public NTP service uses leap-smear time; avoid mixing it with non-smeared sources when comparing results. See [Google Public NTP](https://developers.google.com/time/faq).

## Checks

The tests and help command are included but could not be run in the authoring environment because the Go toolchain was unavailable. Run them locally with Go 1.22 or newer:

```sh
go test ./...
go run ./cmd/ntp-drift-check --help
```

# Sample amqsevt

This program essentially replicates the amqsevt command provided with MQ. It was originally written primarily as a test
of the mqiPCFstr.go functions, but evolved until it was a complete alternative.

The `go.mod` file points at the locally cloned repository for the `ibmmq` package instead of using the published
package. That was because I was changing the package and needed to pick up those changes.

## Differences from the C program
* Only JSON output is available, not the "English-ish" text version
* Object names are given on the command line as a single comma-separated list instead of multiple repeated flags
* The ordering of JSON fields is likely different because Go's `map` type does not give any order guarantees
* Fields may be spelled slightly differently
* The event messages can be written directly to an OpenTelemetry logging service. Available exporters include
  `stdout`, OTLP/http and OTLP/gRPC.

## Testing
There is an `evtTest.sh` script in this directory that can be used as the basis of your own testing. It builds and runs
the program, and executes various steps to create a consistent set of events that can be formatted.

But it probably can't be used unchanged.

* It assumes a local queue manager called `EVENTS`
* It uses a sudo-like program `asid` that sets a specific userid/group
* Generation of activity trace via topics uses another queue manager that I know, on my system, is doing work.

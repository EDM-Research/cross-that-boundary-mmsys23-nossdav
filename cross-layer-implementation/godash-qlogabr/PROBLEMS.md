# Problems encountered

- older quic-go version was used, resulted in error
  - fix: upgrade quic-go version
- using a new connection/client for every request
  - fix: singleton pattern for client
- quic-go was set up to only allowed secure connections
  - fix: set insecure connection flag to true
- quic-go was not set up to generate qlogs
  - fix: add tracer to quic client
- calculate RTT before actual request -> ignores response -> connection is unused -> DATA_BLOCKED
  - fix: calculate RTT when client does request
- stream function is recursive, might cause stack overflow
- maxHeight parameter says which representations should be ignored, this is done using globals, if multiple mime types are available in the MPD, every mime type will overwrite the global values, resulting is only using the values of the last mime type parsed
  - fix: use slices instead of single values
- int is used instead of time.Duration, not every time variable uses the same unit, many conversions are needed
  - time.Duration values are casted into an int by calling Nanoseconds() and then converting into wanted unit

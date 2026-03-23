# Contributing to the CoreSDK Go SDK

## Setup

```bash
git clone git@github.com:coresdk-dev/sdk-go.git && cd sdk-go
go mod download
```

## Running tests

```bash
make test-unit    # no sidecar needed
make test         # all tests (requires sidecar on localhost:50051)
make check        # vet + test
```

## Adding a new SDK method

1. Add the method to `sdk.go` (the `SDK` interface-like struct)
2. Add the gRPC wire call in `client.go` — follow the raw framing pattern
3. Add a stub to `MockSDK` in `testing.go`
4. Add a test in `sdk_test.go`
5. Document in `README.md`

See [core-sdk CONTRIBUTING](https://github.com/coresdk-dev/core-sdk/blob/develop/CONTRIBUTING.md) for the full architecture guide.

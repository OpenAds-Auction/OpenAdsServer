package version

// Ver holds the version derived from the latest git tag
// Populated using:
//
//	go build -ldflags "-X github.com/prebid/prebid-server/version.Ver=`git describe --tags | sed 's/^v//`"
//
// Populated automatically at build / releases in the Docker image
var Ver string

// VerUnknown is the version used if Ver has not been set by ldflags.
const VerUnknown = "unknown"

// Rev holds binary revision string
// Populated using:
//
//	go build -ldflags "-X github.com/prebid/prebid-server/version.Rev=`git rev-parse --short HEAD`"
//
// Populated automatically at build / releases in the Docker image
// See issue #559
var Rev string

// BuildSignature holds the build signature string
// Populated using:
//
//	go build -ldflags "-X github.com/prebid/prebid-server/version.BuildSignature=<signature-value>"
//
// Used for build attestation and verification
var BuildSignature string

// BuildTimestamp holds the build timestamp string
// Populated using:
//
//	go build -ldflags "-X github.com/prebid/prebid-server/version.BuildTimestamp=<timestamp-value>"
//
// Used for build attestation and verification
var BuildTimestamp string

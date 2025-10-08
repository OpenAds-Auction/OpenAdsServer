package endpoints

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/prebid/prebid-server/v3/util/jsonutil"
	"github.com/prebid/prebid-server/v3/version"
)

const attestationEndpointValueNotSet = "not-set"

// NewAttestationEndpoint returns build signature information for attestation purposes
func NewAttestationEndpoint() http.HandlerFunc {
	response, err := prepareAttestationEndpointResponse()
	if err != nil {
		glog.Fatalf("error creating /attestation endpoint response: %v", err)
	}

	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	}
}

func prepareAttestationEndpointResponse() (json.RawMessage, error) {
	buildSignature := version.BuildSignature
	if buildSignature == "" {
		buildSignature = attestationEndpointValueNotSet
	}

	versionStr := version.Ver
	if versionStr == "" {
		versionStr = versionEndpointValueNotSet
	}

	revision := version.Rev
	if revision == "" {
		revision = versionEndpointValueNotSet
	}

	// Create the signature payload for verification
	signaturePayload := ""
	buildTimestamp := version.BuildTimestamp
	if buildTimestamp == "" {
		buildTimestamp = attestationEndpointValueNotSet
	}

	if buildSignature != attestationEndpointValueNotSet && revision != versionEndpointValueNotSet && buildTimestamp != attestationEndpointValueNotSet {
		// Create the actual payload that was signed: <commit-hash>:<timestamp>:prebid-server-build
		signaturePayload = fmt.Sprintf("%s:%s:prebid-server-build", revision, buildTimestamp)
	}

	return jsonutil.Marshal(struct {
		BuildSignature   string `json:"build_signature"`
		Version          string `json:"version"`
		Revision         string `json:"revision"`
		SignaturePayload string `json:"signature_payload"`
		PayloadFormat    string `json:"payload_format"`
	}{
		BuildSignature:   buildSignature,
		Version:          versionStr,
		Revision:         revision,
		SignaturePayload: signaturePayload,
		PayloadFormat:    "<commit-hash>:<timestamp>:prebid-server-build",
	})
}

package signatures

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/hooks/hookstage"
	"github.com/prebid/prebid-server/v3/modules/moduledeps"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFetcher struct {
	response []SignatureWrapper
	err      error
}

func (m *mockFetcher) Fetch(ctx context.Context, body []byte) ([]SignatureWrapper, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestBuilder(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid UDS config with explicit transport",
			config: `{
				"transport": "uds",
				"base_path": "/var/run/test.sock",
				"request_path": "/test/path"
			}`,
			expectError: false,
		},
		{
			name: "valid UDS config with reject_on_failure",
			config: `{
				"transport": "uds",
				"base_path": "/var/run/test.sock",
				"request_path": "/test/path",
				"reject_on_failure": true
			}`,
			expectError: false,
		},
		{
			name: "valid TCP config",
			config: `{
				"transport": "tcp",
				"base_path": "localhost:8080",
				"request_path": "/test/path"
			}`,
			expectError: false,
		},
		{
			name: "valid TCP config with reject_on_failure",
			config: `{
				"transport": "tcp",
				"base_path": "localhost:8080",
				"request_path": "/test/path",
				"reject_on_failure": true
			}`,
			expectError: false,
		},
		{
			name: "request_path without leading slash gets normalized",
			config: `{
				"transport": "tcp",
				"base_path": "localhost:8080",
				"request_path": "test/path"
			}`,
			expectError: false,
		},
		{
			name: "TCP base_path with http scheme is allowed",
			config: `{
				"transport": "tcp",
				"base_path": "http://localhost:8080",
				"request_path": "/test/path"
			}`,
			expectError: false,
		},
		{
			name: "Missing transport",
			config: `{
				"base_path": "/var/run/test.sock",
				"request_path": "/test/path"
			}`,
			expectError: true,
		},
		{
			name:        "missing base_path",
			config:      `{"transport": "uds", "request_path": "/test/path"}`,
			expectError: true,
			errorMsg:    "base_path is required",
		},
		{
			name:        "missing request_path",
			config:      `{"transport": "uds", "base_path": "/var/run/test.sock"}`,
			expectError: true,
			errorMsg:    "request_path is required",
		},
		{
			name:        "invalid transport",
			config:      `{"transport": "invalid", "base_path": "/test", "request_path": "/path"}`,
			expectError: true,
			errorMsg:    "invalid transport",
		},
		{
			name:        "invalid JSON",
			config:      `{invalid}`,
			expectError: true,
			errorMsg:    "failed to parse config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, err := Builder(json.RawMessage(tt.config), moduledeps.ModuleDeps{})

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, module)
			}
		})
	}
}

func TestHandleBidderRequestHook_Success(t *testing.T) {
	tests := []struct {
		name        string
		initialExt  json.RawMessage
		mockResponse []SignatureWrapper
		expectedSig Signature
	}{
		{
			name:       "add int_sigs to nil ext",
			initialExt: nil,
			mockResponse: []SignatureWrapper{
				{Name: "testbidder", SIS: Signature{Envelope: "test-envelope", Source: "thetradedesk.com"}},
			},
			expectedSig: Signature{
				Envelope: "test-envelope",
				Source:   "thetradedesk.com",
			},
		},
		{
			name:       "add int_sigs to empty ext",
			initialExt: json.RawMessage(`{}`),
			mockResponse: []SignatureWrapper{
				{Name: "testbidder", SIS: Signature{Envelope: "envelope-1", Source: "source-1"}},
			},
			expectedSig: Signature{
				Envelope: "envelope-1",
				Source:   "source-1",
			},
		},
		{
			name:       "add int_sigs to existing ext",
			initialExt: json.RawMessage(`{"prebid": {"debug": true}}`),
			mockResponse: []SignatureWrapper{
				{Name: "testbidder", SIS: Signature{Envelope: "envelope-2", Source: "source-2"}},
			},
			expectedSig: Signature{
				Envelope: "envelope-2",
				Source:   "source-2",
			},
		},
		{
			name:       "replace openads with int_sigs",
			initialExt: json.RawMessage(`{"openads": 1, "prebid": {"debug": true}}`),
			mockResponse: []SignatureWrapper{
				{Name: "testbidder", SIS: Signature{Envelope: "envelope-3", Source: "source-3"}},
			},
			expectedSig: Signature{
				Envelope: "envelope-3",
				Source:   "source-3",
			},
		},
		{
			name:       "ignore extra demand sources",
			initialExt: json.RawMessage(`{}`),
			mockResponse: []SignatureWrapper{
				{Name: "testbidder", SIS: Signature{Envelope: "test-envelope", Source: "thetradedesk.com"}},
				{Name: "casale", SIS: Signature{Envelope: "extra-envelope", Source: "extra-source"}},
			},
			expectedSig: Signature{
				Envelope: "test-envelope",
				Source:   "thetradedesk.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := Module{
				cfg: &Config{
					Transport:       TransportUDS,
					BasePath:        "/test.sock",
					RequestPath:     "/test",
					RejectOnFailure: false,
					Version:         SchemaVersion,
				},
				fetcher: &mockFetcher{
					response: tt.mockResponse,
					err:      nil,
				},
			}

			bidRequest := &openrtb2.BidRequest{
				ID: "test-request",
				Imp: []openrtb2.Imp{
					{ID: "test-imp"},
				},
				Ext: tt.initialExt,
			}

			requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}

			payload := hookstage.BidderRequestPayload{
				Request: requestWrapper,
				Bidder:  "testbidder",
			}

			result, err := module.HandleBidderRequestHook(
				context.Background(),
				hookstage.ModuleInvocationContext{},
				payload,
			)

			require.NoError(t, err)
			assert.False(t, result.Reject)
			assert.Equal(t, 0, result.NbrCode)

			// Apply the mutations to get the final result
			finalPayload := payload
			for _, mutation := range result.ChangeSet.Mutations() {
				finalPayload, err = mutation.Apply(finalPayload)
				require.NoError(t, err)
			}

			require.NoError(t, finalPayload.Request.RebuildRequest())

			// Verify openads field was added
			var extMap map[string]json.RawMessage
			err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
			require.NoError(t, err)

			var openadsExt OpenAdsExt
			err = json.Unmarshal(extMap["openads"], &openadsExt)
			require.NoError(t, err)

			assert.Equal(t, SchemaVersion, openadsExt.Version)
			require.Len(t, openadsExt.IntSigs, 1)
			assert.Equal(t, tt.expectedSig, openadsExt.IntSigs[0])

			// Verify other fields are preserved if they existed
			if len(tt.initialExt) > 2 {
				var originalExt map[string]json.RawMessage
				json.Unmarshal(tt.initialExt, &originalExt)

				for key := range originalExt {
					if key != "openads" {
						_, exists := extMap[key]
						assert.True(t, exists, "existing field %s should be preserved", key)
					}
				}
			}
		})
	}
}

func TestHandleBidderRequestHook_MissingDemandSource(t *testing.T) {
	tests := []struct {
		name            string
		mockResponse    []SignatureWrapper
		rejectOnFailure bool
		expectReject    bool
		expectErr       string
	}{
		{
			name: "missing demandSource - soft mode",
			mockResponse: []SignatureWrapper{
				{Name: "other-bidder", SIS: Signature{Envelope: "envelope", Source: "source"}},
			},
			rejectOnFailure: false,
			expectReject:    false,
			expectErr:       "missing demandSources in sidecar response: [testbidder]",
		},
		{
			name: "missing demandSource - reject mode",
			mockResponse: []SignatureWrapper{
				{Name: "other-bidder", SIS: Signature{Envelope: "envelope", Source: "source"}},
			},
			rejectOnFailure: true,
			expectReject:    true,
			expectErr:       "missing demandSources in sidecar response: [testbidder]",
		},
		{
			name:            "empty response - soft mode",
			mockResponse:    []SignatureWrapper{},
			rejectOnFailure: false,
			expectReject:    false,
			expectErr:       "missing demandSources in sidecar response: [testbidder]",
		},
		{
			name:            "empty response - reject mode",
			mockResponse:    []SignatureWrapper{},
			rejectOnFailure: true,
			expectReject:    true,
			expectErr:       "missing demandSources in sidecar response: [testbidder]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := Module{
				cfg: &Config{
					Transport:       TransportUDS,
					BasePath:        "/test.sock",
					RequestPath:     "/test",
					RejectOnFailure: tt.rejectOnFailure,
					Version:         SchemaVersion,
				},
				fetcher: &mockFetcher{
					response: tt.mockResponse,
					err:      nil,
				},
			}

			bidRequest := &openrtb2.BidRequest{
				ID:  "test-request",
				Ext: json.RawMessage(`{"prebid": {"debug": true}}`),
			}

			requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}

			payload := hookstage.BidderRequestPayload{
				Request: requestWrapper,
				Bidder:  "testbidder",
			}

			result, err := module.HandleBidderRequestHook(
				context.Background(),
				hookstage.ModuleInvocationContext{},
				payload,
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErr)
			assert.Equal(t, tt.expectReject, result.Reject)

			if tt.expectReject {
				assert.Equal(t, NbrCodeServiceUnavailable, result.NbrCode)
				assert.Len(t, result.ChangeSet.Mutations(), 0)
			} else {
				assert.Equal(t, 0, result.NbrCode)
				// Should still set openads with empty int_sigs
				mutations := result.ChangeSet.Mutations()
				assert.Len(t, mutations, 1, "should have one mutation for soft-fail")

				finalPayload := payload
				for _, mutation := range mutations {
					finalPayload, err = mutation.Apply(finalPayload)
					require.NoError(t, err)
				}

				require.NoError(t, finalPayload.Request.RebuildRequest())

				var extMap map[string]json.RawMessage
				err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
				require.NoError(t, err)

				var openadsExt OpenAdsExt
				err = json.Unmarshal(extMap["openads"], &openadsExt)
				require.NoError(t, err)

				assert.Equal(t, SchemaVersion, openadsExt.Version)
				assert.Empty(t, openadsExt.IntSigs)
			}
		})
	}
}

func TestHandleBidderRequestHook_FailureSoftMode(t *testing.T) {
	tests := []struct {
		name      string
		fetchErr  error
		expectErr string
	}{
		{
			name:      "network error",
			fetchErr:  errors.New("connection refused"),
			expectErr: "sidecar fetch: connection refused",
		},
		{
			name:      "timeout error",
			fetchErr:  errors.New("context deadline exceeded"),
			expectErr: "sidecar fetch: context deadline exceeded",
		},
		{
			name:      "invalid response",
			fetchErr:  errors.New("unexpected status code: 500"),
			expectErr: "sidecar fetch: unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := Module{
				cfg: &Config{
					Transport:       TransportUDS,
					BasePath:        "/test.sock",
					RequestPath:     "/test",
					RejectOnFailure: false,
					Version:         SchemaVersion,
				},
				fetcher: &mockFetcher{
					response: nil,
					err:      tt.fetchErr,
				},
			}

			bidRequest := &openrtb2.BidRequest{
				ID:  "test-request",
				Ext: json.RawMessage(`{"prebid": {"debug": true}}`),
			}

			requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}

			payload := hookstage.BidderRequestPayload{
				Request: requestWrapper,
				Bidder:  "testbidder",
			}

			result, err := module.HandleBidderRequestHook(
				context.Background(),
				hookstage.ModuleInvocationContext{},
				payload,
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErr)

			assert.False(t, result.Reject)
			assert.Equal(t, 0, result.NbrCode)

			mutations := result.ChangeSet.Mutations()
			assert.Len(t, mutations, 1, "should have one mutation for soft-fail")

			finalPayload := payload
			for _, mutation := range mutations {
				finalPayload, err = mutation.Apply(finalPayload)
				require.NoError(t, err)
			}

			require.NoError(t, finalPayload.Request.RebuildRequest())

			// Verify openads field was added with version and empty int_sigs
			var extMap map[string]json.RawMessage
			err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
			require.NoError(t, err)

			var openadsExt OpenAdsExt
			err = json.Unmarshal(extMap["openads"], &openadsExt)
			require.NoError(t, err)

			assert.Equal(t, SchemaVersion, openadsExt.Version)
			assert.Empty(t, openadsExt.IntSigs)

			// Verify other fields are preserved
			var prebidExt map[string]interface{}
			err = json.Unmarshal(extMap["prebid"], &prebidExt)
			require.NoError(t, err)
			assert.Equal(t, true, prebidExt["debug"])
		})
	}
}

func TestHandleBidderRequestHook_FailureRejectMode(t *testing.T) {
	tests := []struct {
		name      string
		fetchErr  error
		expectErr string
	}{
		{
			name:      "network error with rejection",
			fetchErr:  errors.New("connection refused"),
			expectErr: "sidecar fetch: connection refused",
		},
		{
			name:      "timeout error with rejection",
			fetchErr:  errors.New("context deadline exceeded"),
			expectErr: "sidecar fetch: context deadline exceeded",
		},
		{
			name:      "invalid status with rejection",
			fetchErr:  errors.New("unexpected status code: 500"),
			expectErr: "sidecar fetch: unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := Module{
				cfg: &Config{
					Transport:       TransportUDS,
					BasePath:        "/test.sock",
					RequestPath:     "/test",
					RejectOnFailure: true,
					Version:         SchemaVersion,
				},
				fetcher: &mockFetcher{
					response: nil,
					err:      tt.fetchErr,
				},
			}

			bidRequest := &openrtb2.BidRequest{
				ID:  "test-request",
				Ext: json.RawMessage(`{"prebid": {"debug": true}}`),
			}

			requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}

			payload := hookstage.BidderRequestPayload{
				Request: requestWrapper,
				Bidder:  "testbidder",
			}

			result, err := module.HandleBidderRequestHook(
				context.Background(),
				hookstage.ModuleInvocationContext{},
				payload,
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErr)

			// In reject mode, request SHOULD be rejected
			assert.True(t, result.Reject)
			assert.Equal(t, NbrCodeServiceUnavailable, result.NbrCode)

			// Verify no mutations were applied
			mutations := result.ChangeSet.Mutations()
			assert.Len(t, mutations, 0)
		})
	}
}

func TestHandleBidderRequestHook_NilRequest(t *testing.T) {
	module := Module{
		cfg: &Config{
			Transport:       TransportUDS,
			BasePath:        "/test.sock",
			RequestPath:     "/test",
			RejectOnFailure: false,
			Version:         SchemaVersion,
		},
		fetcher: &mockFetcher{
			response: []SignatureWrapper{},
			err:      nil,
		},
	}

	payload := hookstage.BidderRequestPayload{
		Request: nil,
		Bidder:  "testbidder",
	}

	result, err := module.HandleBidderRequestHook(
		context.Background(),
		hookstage.ModuleInvocationContext{},
		payload,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "payload contains a nil bid request")
	assert.False(t, result.Reject)
}

func TestHandleBidderRequestHook_MutationTracking(t *testing.T) {
	module := Module{
		cfg: &Config{
			Transport:       TransportUDS,
			BasePath:        "/test.sock",
			RequestPath:     "/test",
			RejectOnFailure: false,
			Version:         SchemaVersion,
		},
		fetcher: &mockFetcher{
			response: []SignatureWrapper{
				{Name: "testbidder", SIS: Signature{Envelope: "test-signature", Source: "test-source"}},
			},
			err: nil,
		},
	}

	bidRequest := &openrtb2.BidRequest{
		ID:  "test-request",
		Ext: json.RawMessage(`{}`),
	}

	requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}

	payload := hookstage.BidderRequestPayload{
		Request: requestWrapper,
		Bidder:  "testbidder",
	}

	result, err := module.HandleBidderRequestHook(
		context.Background(),
		hookstage.ModuleInvocationContext{},
		payload,
	)

	require.NoError(t, err)
	assert.False(t, result.Reject)

	// Verify one mutation was tracked
	mutations := result.ChangeSet.Mutations()
	assert.Len(t, mutations, 1)

	mutation := mutations[0]
	modifiedPayload, err := mutation.Apply(payload)
	require.NoError(t, err)

	require.NoError(t, modifiedPayload.Request.RebuildRequest())

	// Verify the mutation actually worked
	var extMap map[string]json.RawMessage
	err = json.Unmarshal(modifiedPayload.Request.BidRequest.Ext, &extMap)
	require.NoError(t, err)

	var openadsExt OpenAdsExt
	err = json.Unmarshal(extMap["openads"], &openadsExt)
	require.NoError(t, err)

	assert.Equal(t, SchemaVersion, openadsExt.Version)
	require.Len(t, openadsExt.IntSigs, 1)
	
	expectedSig := Signature{
		Envelope: "test-signature",
		Source:   "test-source",
	}
	assert.Equal(t, expectedSig, openadsExt.IntSigs[0])
}

func TestHandleBidderRequestHook_InvalidJSONResponse(t *testing.T) {
	tests := []struct {
		name            string
		mockError       error
		rejectOnFailure bool
		expectReject    bool
		expectNbrCode   int
	}{
		{
			name:            "invalid JSON - soft mode",
			mockError:       errors.New("invalid JSON array in response: invalid character"),
			rejectOnFailure: false,
			expectReject:    false,
			expectNbrCode:   0,
		},
		{
			name:            "invalid JSON - reject mode",
			mockError:       errors.New("invalid JSON array in response: invalid character"),
			rejectOnFailure: true,
			expectReject:    true,
			expectNbrCode:   NbrCodeServiceUnavailable,
		},
		{
			name:            "not an array - soft mode",
			mockError:       errors.New("invalid JSON array in response: json: cannot unmarshal object"),
			rejectOnFailure: false,
			expectReject:    false,
			expectNbrCode:   0,
		},
		{
			name:            "not an array - reject mode",
			mockError:       errors.New("invalid JSON array in response: json: cannot unmarshal object"),
			rejectOnFailure: true,
			expectReject:    true,
			expectNbrCode:   NbrCodeServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := Module{
				cfg: &Config{
					Transport:       TransportUDS,
					BasePath:        "/test.sock",
					RequestPath:     "/test",
					RejectOnFailure: tt.rejectOnFailure,
					Version:         SchemaVersion,
				},
				fetcher: &mockFetcher{
					response: nil,
					err:      tt.mockError,
				},
			}

			bidRequest := &openrtb2.BidRequest{
				ID:  "test-request",
				Ext: json.RawMessage(`{}`),
			}

			requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}

			payload := hookstage.BidderRequestPayload{
				Request: requestWrapper,
				Bidder:  "testbidder",
			}

			result, err := module.HandleBidderRequestHook(
				context.Background(),
				hookstage.ModuleInvocationContext{},
				payload,
			)

			require.Error(t, err)
			assert.Equal(t, tt.expectReject, result.Reject)
			assert.Equal(t, tt.expectNbrCode, result.NbrCode)
		})
	}
}

func TestTCPIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var envelope signatureRequest
		err = json.Unmarshal(body, &envelope)
		require.NoError(t, err)

		assert.Equal(t, []string{"testbidder"}, envelope.DemandSources)

		requestBodyJSON, err := json.Marshal(envelope.RequestBody)
		require.NoError(t, err)

		var bidRequest openrtb2.BidRequest
		err = json.Unmarshal(requestBodyJSON, &bidRequest)
		require.NoError(t, err)
		assert.Equal(t, "test-request-id", bidRequest.ID)

		w.WriteHeader(http.StatusOK)
		response := []SignatureWrapper{
			{Name: "testbidder", SIS: Signature{Envelope: "sig-1", Source: "source-1"}},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	basePath := server.URL

	cfg := &Config{
		Transport:       TransportTCP,
		BasePath:        basePath,
		RequestPath:     "/",
		RejectOnFailure: false,
		Version:         SchemaVersion,
	}

	fetcher, err := newFetcher(cfg)
	require.NoError(t, err)

	module := Module{
		cfg:     cfg,
		fetcher: fetcher,
	}

	bidRequest := &openrtb2.BidRequest{
		ID:  "test-request-id",
		Ext: json.RawMessage(`{}`),
	}
	requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}
	payload := hookstage.BidderRequestPayload{
		Request: requestWrapper,
		Bidder:  "testbidder",
	}

	// Execute hook
	result, err := module.HandleBidderRequestHook(
		context.Background(),
		hookstage.ModuleInvocationContext{},
		payload,
	)

	require.NoError(t, err)
	assert.False(t, result.Reject)

	// Apply mutations and verify result
	finalPayload := payload
	for _, mutation := range result.ChangeSet.Mutations() {
		finalPayload, err = mutation.Apply(finalPayload)
		require.NoError(t, err)
	}

	require.NoError(t, finalPayload.Request.RebuildRequest())

	var extMap map[string]json.RawMessage
	err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
	require.NoError(t, err)

	var openadsExt OpenAdsExt
	err = json.Unmarshal(extMap["openads"], &openadsExt)
	require.NoError(t, err)

	assert.Equal(t, SchemaVersion, openadsExt.Version)
	require.Len(t, openadsExt.IntSigs, 1)
	
	expectedSig := Signature{
		Envelope: "sig-1",
		Source:   "source-1",
	}
	assert.Equal(t, expectedSig, openadsExt.IntSigs[0])
}

func TestUDSIntegration(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer listener.Close()
	defer os.Remove(socketPath)

	// Start HTTP server over UDS
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var envelope signatureRequest
			err = json.Unmarshal(body, &envelope)
			require.NoError(t, err)

			assert.Equal(t, []string{"testbidder"}, envelope.DemandSources)

			requestBodyJSON, err := json.Marshal(envelope.RequestBody)
			require.NoError(t, err)

			var bidRequest openrtb2.BidRequest
			err = json.Unmarshal(requestBodyJSON, &bidRequest)
			require.NoError(t, err)
			assert.Equal(t, "test-request-id", bidRequest.ID)

			w.WriteHeader(http.StatusOK)
			response := []SignatureWrapper{
				{Name: "testbidder", SIS: Signature{Envelope: "sig-1", Source: "source-1"}},
			}
			json.NewEncoder(w).Encode(response)
		}),
	}

	go server.Serve(listener)
	defer server.Close()

	cfg := &Config{
		Transport:       TransportUDS,
		BasePath:        socketPath,
		RequestPath:     "/",
		RejectOnFailure: false,
		Version:         SchemaVersion,
	}

	fetcher, err := newFetcher(cfg)
	require.NoError(t, err)

	module := Module{
		cfg:     cfg,
		fetcher: fetcher,
	}

	bidRequest := &openrtb2.BidRequest{
		ID:  "test-request-id",
		Ext: json.RawMessage(`{"prebid": {"debug": true}}`),
	}
	requestWrapper := &openrtb_ext.RequestWrapper{BidRequest: bidRequest}
	payload := hookstage.BidderRequestPayload{
		Request: requestWrapper,
		Bidder:  "testbidder",
	}

	// Execute hook
	result, err := module.HandleBidderRequestHook(
		context.Background(),
		hookstage.ModuleInvocationContext{},
		payload,
	)

	require.NoError(t, err)
	assert.False(t, result.Reject)

	// Apply mutations and verify result
	finalPayload := payload
	for _, mutation := range result.ChangeSet.Mutations() {
		finalPayload, err = mutation.Apply(finalPayload)
		require.NoError(t, err)
	}

	require.NoError(t, finalPayload.Request.RebuildRequest())

	var extMap map[string]json.RawMessage
	err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
	require.NoError(t, err)

	var openadsExt OpenAdsExt
	err = json.Unmarshal(extMap["openads"], &openadsExt)
	require.NoError(t, err)

	assert.Equal(t, SchemaVersion, openadsExt.Version)
	require.Len(t, openadsExt.IntSigs, 1)
	
	expectedSig := Signature{
		Envelope: "sig-1",
		Source:   "source-1",
	}
	assert.Equal(t, expectedSig, openadsExt.IntSigs[0])
	assert.NotEmpty(t, extMap["prebid"])
}

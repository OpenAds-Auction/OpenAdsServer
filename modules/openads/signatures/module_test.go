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
	response []interface{}
	err      error
}

func (m *mockFetcher) Fetch(ctx context.Context, body []byte) ([]interface{}, error) {
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
		name         string
		initialExt   json.RawMessage
		mockResponse []interface{}
	}{
		{
			name:         "add int_sigs to nil ext",
			initialExt:   nil,
			mockResponse: []interface{}{"signature-1", "signature-2", "signature-3"},
		},
		{
			name:         "add int_sigs to empty ext",
			initialExt:   json.RawMessage(`{}`),
			mockResponse: []interface{}{"signature-1"},
		},
		{
			name:         "add int_sigs to existing ext",
			initialExt:   json.RawMessage(`{"prebid": {"debug": true}}`),
			mockResponse: []interface{}{"signature-1"},
		},
		{
			name:         "replace openads with int_sigs",
			initialExt:   json.RawMessage(`{"openads": 1, "prebid": {"debug": true}}`),
			mockResponse: []interface{}{"signature-1"},
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

			// Verify openads field was added
			var extMap map[string]interface{}
			err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
			require.NoError(t, err)

			expected := map[string]interface{}{
				"version":  float64(SchemaVersion),
				"int_sigs": tt.mockResponse,
			}
			assert.Equal(t, expected, extMap["openads"])

			// Verify other fields are preserved if they existed
			if len(tt.initialExt) > 2 {
				var originalExt map[string]interface{}
				json.Unmarshal(tt.initialExt, &originalExt)

				for key, expectedValue := range originalExt {
					if key != "openads" {
						actualValue, exists := extMap[key]
						assert.True(t, exists, "existing field %s should be preserved", key)
						assert.Equal(t, expectedValue, actualValue)
					}
				}
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

			// Verify openads field was added with version and empty int_sigs
			var extMap map[string]interface{}
			err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
			require.NoError(t, err)

			expected := map[string]interface{}{
				"version":  float64(SchemaVersion),
				"int_sigs": []interface{}{},
			}
			assert.Equal(t, expected, extMap["openads"])

			// Verify other fields are preserved
			prebidMap := extMap["prebid"].(map[string]interface{})
			assert.Equal(t, true, prebidMap["debug"])
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
			response: []interface{}{},
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
			response: []interface{}{"test-signature"},
			err:      nil,
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

	// Verify the mutation actually worked
	var extMap map[string]interface{}
	err = json.Unmarshal(modifiedPayload.Request.BidRequest.Ext, &extMap)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"version":  float64(SchemaVersion),
		"int_sigs": []interface{}{"test-signature"},
	}
	assert.Equal(t, expected, extMap["openads"])
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
		w.Write([]byte(`["sig-1", "sig-2"]`))
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

	var extMap map[string]interface{}
	err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"version":  float64(SchemaVersion),
		"int_sigs": []interface{}{"sig-1", "sig-2"},
	}
	assert.Equal(t, expected, extMap["openads"])
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
			w.Write([]byte(`["sig-1", "sig-2"]`))
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

	var extMap map[string]interface{}
	err = json.Unmarshal(finalPayload.Request.BidRequest.Ext, &extMap)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"version":  float64(SchemaVersion),
		"int_sigs": []interface{}{"sig-1", "sig-2"},
	}
	assert.Equal(t, expected, extMap["openads"])
	assert.NotNil(t, extMap["prebid"])
}

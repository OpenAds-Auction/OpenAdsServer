package auctionaudit

import (
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
)

func TestParseCompression(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    sarama.CompressionCodec
		expectError bool
	}{
		{
			name:     "empty string defaults to none",
			input:    "",
			expected: sarama.CompressionNone,
		},
		{
			name:     "none",
			input:    "none",
			expected: sarama.CompressionNone,
		},
		{
			name:     "snappy",
			input:    "snappy",
			expected: sarama.CompressionSnappy,
		},
		{
			name:     "gzip",
			input:    "gzip",
			expected: sarama.CompressionGZIP,
		},
		{
			name:     "lz4",
			input:    "lz4",
			expected: sarama.CompressionLZ4,
		},
		{
			name:     "zstd",
			input:    "zstd",
			expected: sarama.CompressionZSTD,
		},
		{
			name:        "invalid compression",
			input:       "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCompression(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

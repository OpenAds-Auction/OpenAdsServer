package ssl

import (
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"
)

<<<<<<< HEAD
func TestCertsFromFilePoolExists(t *testing.T) {
	// Load hardcoded certificates found in ssl.go
	certPool := GetRootCAPool()
=======
func TestAppendPEMFileToCertPool(t *testing.T) {
	t.Run("append-to-empty", func(t *testing.T) {
		var certPool *x509.CertPool = nil
>>>>>>> c6afd83c (Deprecate Embedded Certs (#4625))

		certificatesFile := "mockcertificates/mock-certs.pem"
		certPool, err := AppendPEMFileToCertPool(certPool, certificatesFile)

		require.NoError(t, err)
		subjects := certPool.Subjects()
		assert.Equal(t, len(subjects), 1)
	})

	t.Run("fail", func(t *testing.T) {
		var certPool *x509.CertPool

		certificatesFile := "mockcertificates/NO-FILE.pem"
		_, err := AppendPEMFileToCertPool(certPool, certificatesFile)

		// expect an error from a file which doesn't exist
		assert.Error(t, err)
	})
}

package ssl

import (
	"crypto/x509"
	"fmt"
	"os"
)

<<<<<<< HEAD
// from https://medium.com/@kelseyhightower/optimizing-docker-images-for-static-binaries-b5696e26eb07

var pool *x509.CertPool

func GetRootCAPool() *x509.CertPool {
	if pool == nil {
		pool = x509.NewCertPool()
		pool.AppendCertsFromPEM(pemCerts)
	}
	return pool
}

// Appends certificates to the `x509.CertPool` from a `.pem` private local file. On many Linux
// systems, /etc/ssl/cert.pem will contain the system wide set but in our case, we'll pull
// the certificate file path from the `Configuration` struct
func AppendPEMFileToRootCAPool(certPool *x509.CertPool, pemFileName string) (*x509.CertPool, error) {
=======
func CreateCertPool() (*x509.CertPool, error) {
	return x509.SystemCertPool()
}

// AppendPEMFileToCertPool appends certificates from a PEM file to the provided certificate pool.
// This is a helper method intended for use in main startup code to append specific certificates
// to the system certificate pool.
func AppendPEMFileToCertPool(certPool *x509.CertPool, pemFileName string) (*x509.CertPool, error) {
>>>>>>> c6afd83c (Deprecate Embedded Certs (#4625))
	if certPool == nil {
		certPool = x509.NewCertPool()
	}

	if pemFileName != "" {
		pemCerts, err := os.ReadFile(pemFileName)
		if err != nil {
			return certPool, fmt.Errorf("Failed to read file %s: %v", pemFileName, err)
		}

		certPool.AppendCertsFromPEM(pemCerts)
	}

	return certPool, nil
}

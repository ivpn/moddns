package server

import (
	"crypto/tls"
	"fmt"
	"os"
)

// newTLSConfig returns a TLS config that includes one or more certificates.
// certPaths and keyPaths are parallel slices; each pair is loaded so that
// Go's crypto/tls can auto-select the matching certificate by SNI.
func newTLSConfig(minVersion, maxVersion float32, certPaths, keyPaths []string) (*tls.Config, error) {
	if len(certPaths) == 0 {
		return nil, fmt.Errorf("no TLS certificate paths provided")
	}
	if len(certPaths) != len(keyPaths) {
		return nil, fmt.Errorf("TLS_CERT_PATH has %d entries but TLS_KEY_PATH has %d; they must match", len(certPaths), len(keyPaths))
	}

	// Set default TLS min/max versions
	tlsMinVersion := tls.VersionTLS10 // Default for crypto/tls
	tlsMaxVersion := tls.VersionTLS13 // Default for crypto/tls
	switch minVersion {
	case 1.1:
		tlsMinVersion = tls.VersionTLS11
	case 1.2:
		tlsMinVersion = tls.VersionTLS12
	case 1.3:
		tlsMinVersion = tls.VersionTLS13
	}
	switch maxVersion {
	case 1.0:
		tlsMaxVersion = tls.VersionTLS10
	case 1.1:
		tlsMaxVersion = tls.VersionTLS11
	case 1.2:
		tlsMaxVersion = tls.VersionTLS12
	}

	certs := make([]tls.Certificate, 0, len(certPaths))
	for i := range certPaths {
		cert, err := loadX509KeyPair(certPaths[i], keyPaths[i])
		if err != nil {
			return nil, fmt.Errorf("could not load TLS cert pair %d (%s): %s", i, certPaths[i], err)
		}
		certs = append(certs, cert)
	}

	// #nosec G402 -- TLS MinVersion is configured by user.
	return &tls.Config{
		Certificates: certs,
		MinVersion:   uint16(tlsMinVersion), // nolint
		MaxVersion:   uint16(tlsMaxVersion), // nolint
	}, nil
}

// loadX509KeyPair reads and parses a public/private key pair from a pair of
// files.  The files must contain PEM encoded data.  The certificate file may
// contain intermediate certificates following the leaf certificate to form a
// certificate chain.  On successful return, Certificate.Leaf will be nil
// because the parsed form of the certificate is not retained.
func loadX509KeyPair(certFile, keyFile string) (crt tls.Certificate, err error) {
	// #nosec G304 -- Trust the file path that is given in the configuration.
	certPEMBlock, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, err
	}

	// #nosec G304 -- Trust the file path that is given in the configuration.
	keyPEMBlock, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(certPEMBlock, keyPEMBlock)
}

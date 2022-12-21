package auth

import (
	"crypto/tls"
)

// GetTLSConfig returns a tls.Config for xDS Client.
func GetTLSConfig(xdsServerName string) (*tls.Config, error) {
	cfg := &tls.Config{
		// Skip this and use JWT for auth in Istiod.
		//ServerName: xdsServerName,
		//RootCAs:            rootCertPool, // used to verify the server certificate.
		//Certificates: nil, // client's certificate
		InsecureSkipVerify: true,
	}
	return cfg, nil
}

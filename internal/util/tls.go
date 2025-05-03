package util

import (
	"crypto/x509"
	"errors"
	"os"
)

func LoadCertPool(rootCAFile string) (*x509.CertPool, error) {
	certBytes, err := os.ReadFile(rootCAFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(certBytes)
	if !ok {
		return nil, errors.New("could not append root certificate to pool")
	}

	return certPool, nil
}

package dtls

//Adapted from https://github.com/gaukas/seed2sdp/blob/master/dtlsCertificate.go

import (
	ecdsa_go "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/refraction-networking/conjure/internal/crypto/ecdsa"
	"github.com/refraction-networking/conjure/internal/crypto/x509"
	"golang.org/x/crypto/hkdf"
)

// getPrivkey creates ECDSA private key used in DTLS Certificates
func getPrivkey(seed []byte) (*ecdsa.PrivateKey, error) {
	randSource := hkdf.New(sha256.New, seed, nil, nil)

	privkey, err := ecdsa.GenerateKey(elliptic.P256(), randSource)
	if err != nil {
		return &ecdsa.PrivateKey{}, err
	}
	return privkey, nil
}

// getPrivkey creates ECDSA private key used in DTLS Certificates
func getPrivkeyGo(seed []byte) (*ecdsa_go.PrivateKey, error) {
	randSource := hkdf.New(sha256.New, seed, nil, nil)

	privkey, err := ecdsa_go.GenerateKey(elliptic.P256(), randSource)
	if err != nil {
		return &ecdsa_go.PrivateKey{}, err
	}

	return privkey, nil
}

// getX509Tpl creates x509 template for x509 Certificates generation used in DTLS Certificates.
func getX509Tpl(seed []byte) (*x509.Certificate, error) {
	randSource := hkdf.New(sha256.New, seed, nil, nil)

	maxBigInt := new(big.Int)
	maxBigInt.Exp(big.NewInt(2), big.NewInt(130), nil).Sub(maxBigInt, big.NewInt(1))
	serialNumber, err := rand.Int(randSource, maxBigInt)
	if err != nil {
		return &x509.Certificate{}, err
	}

	// Make the Certificate valid from UTC today till next month.
	utcNow := time.Now().UTC()
	validFrom := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 0, 0, 0, 0, time.UTC)
	validUntil := validFrom.AddDate(0, 1, 0)

	// random CN
	cnBytes := make([]byte, 8)
	_, err = io.ReadFull(randSource, cnBytes)
	if err != nil {
		return &x509.Certificate{}, fmt.Errorf("failed to generate common name: %w", err)
	}
	cn := hex.EncodeToString(cnBytes)

	return &x509.Certificate{
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		NotBefore:             validFrom,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		NotAfter:              validUntil,
		SerialNumber:          serialNumber,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		Version:               2,
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              []string{cn},
		IsCA:                  true,
	}, nil
}

func newCertificate(seed []byte) (*tls.Certificate, error) {
	privkey, err := getPrivkey(seed)
	if err != nil {
		return &tls.Certificate{}, err
	}

	privkey_go, err := getPrivkeyGo(seed)
	if err != nil {
		return &tls.Certificate{}, err
	}

	tpl, err := getX509Tpl(seed)
	if err != nil {
		return &tls.Certificate{}, err
	}

	randSource := hkdf.New(sha256.New, seed, nil, nil)

	certDER, err := x509.CreateCertificate(randSource, tpl, tpl, privkey.Public(), privkey)
	if err != nil {
		return &tls.Certificate{}, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privkey_go,
	}, nil
}

func certsFromSeed(seed []byte) (*tls.Certificate, *tls.Certificate, error) {
	clientCert, err := newCertificate(seed)
	if err != nil {
		fmt.Printf("error generate cert: %v", err)
		return &tls.Certificate{}, &tls.Certificate{}, fmt.Errorf("error generate cert: %v", err)
	}

	serverCert, err := newCertificate(seed)
	if err != nil {
		return &tls.Certificate{}, &tls.Certificate{}, fmt.Errorf("error generate cert: %v", err)
	}

	return clientCert, serverCert, nil
}

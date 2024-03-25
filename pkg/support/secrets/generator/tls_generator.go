package generator

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"k8s.io/kube-openapi/pkg/validation/strfmt"
)

type TLSGenerator struct {
	algorithm  Algorithm
	size       Size
	lifespan   time.Duration
	commonName string
	hosts      []string
}

var ErrInvalidAlgorithmSize = errors.New("invalid algorithm/size pair")

func NewTLSGenerator(sizeAnnotation, algorithmAnnotation, lifespanAnnotation, commonNameAnnotation string, hostAnnotations []string) (*TLSGenerator, error) {
	size, err := ParseSize(sizeAnnotation)
	if err != nil {
		return nil, err
	}

	algorithm, err := ParseAlgorithm(algorithmAnnotation)
	if err != nil {
		return nil, err
	}

	lifespan, err := strfmt.ParseDuration(lifespanAnnotation)
	if err != nil {
		return nil, err
	}

	g := TLSGenerator{
		algorithm:  algorithm,
		size:       size,
		lifespan:   lifespan,
		commonName: commonNameAnnotation,
		hosts:      hostAnnotations,
	}

	return &g, nil
}

func (t TLSGenerator) generateKeyPair() (any, error) {
	var priv any

	var err error

	switch t.algorithm {
	case ED25519:
		_, priv, err = ed25519.GenerateKey(rand.Reader)
	case RSA:
		priv, err = rsa.GenerateKey(rand.Reader, int(t.size))
	case ECDSA:
		switch t.size {
		case 256:
			priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		case 384:
			priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		case 521:
			priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		}
	default:
		return nil, fmt.Errorf("algorithm: %s %w", t.algorithm, ErrInvalidAlgorithm)
	}

	if err != nil {
		return nil, err
	}

	if priv == nil {
		return nil, fmt.Errorf("algorithm/size: %s/%s %w", t.algorithm, t.size, ErrInvalidAlgorithmSize)
	}

	return priv, nil
}

func (t TLSGenerator) getPublicKey(priv any) any {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey)
	default:
		return nil
	}
}

func (t TLSGenerator) Generate(_ string) (Content, error) {
	priv, err := t.generateKeyPair()
	if err != nil {
		return nil, err
	}

	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	// Also make this cert its own Certificate Authority
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign

	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set.
	// In the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	if _, isRSA := priv.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	notBefore := now()
	notAfter := notBefore.Add(t.lifespan)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 20*8)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	// Add 1 to avoid a serial number of 0
	serialNumber = serialNumber.Add(serialNumber, big.NewInt(1))

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: t.commonName,
		},
		Issuer: pkix.Name{
			CommonName: t.commonName,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		IsCA:        true,
		KeyUsage:    keyUsage,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},

		BasicConstraintsValid: true,
	}

	for _, h := range t.hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, t.getPublicKey(priv), priv)
	if err != nil {
		return nil, err
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}

	var certOut bytes.Buffer
	if err = pem.Encode(&certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, fmt.Errorf("failed to encode X509 certificate in PEM format: %w", err)
	}

	var keyOut bytes.Buffer
	if err = pem.Encode(&keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return nil, fmt.Errorf("failed to encode X509 key in PEM format: %w", err)
	}

	return Content{"tls.crt": certOut.Bytes(), "tls.key": keyOut.Bytes()}, nil
}

package generator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = BeforeSuite(func() {
	now = func() time.Time { return time.Date(2000, time.January, 1, 12, 0, 0, 0, time.UTC) }
})

var _ = AfterSuite(func() {
	now = time.Now
})

var _ = Describe("TLSGenerator", func() {
	Describe("Generate", func() {
		var algorithm string
		var size string
		var lifespan string
		var commonName string
		var hosts []string
		var cert *x509.Certificate

		JustBeforeEach(func() {
			generator, err := NewTLSGenerator(size, algorithm, lifespan, commonName, hosts)
			Expect(err).NotTo(HaveOccurred())

			bytes, err := generator.Generate("tls")
			Expect(err).NotTo(HaveOccurred())

			dir, err := os.MkdirTemp("", "operator")
			Expect(err).NotTo(HaveOccurred())
			socket := dir + "/test.sock"
			defer func() { _ = os.Remove(dir) }()

			server, _, err := startServer(socket, bytes["tls.crt"], bytes["tls.key"])
			Expect(err).NotTo(HaveOccurred())

			body, err := httpsClient(socket, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(body).To(Equal("hello world"))

			_, err = httpsClient(socket, true)
			Expect(err).To(HaveOccurred())

			certs, err := certClient(socket, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(certs).To(HaveLen(1))

			cert = certs[0]

			err = server.Shutdown(context.Background())
			Expect(err).NotTo(HaveOccurred())
		})

		CommonBehavior := func() {
			It("should have the issuer common name", func() {
				Expect(cert.Issuer.CommonName).To(Equal(commonName))
			})
			It("should have the subject common name", func() {
				Expect(cert.Subject.CommonName).To(Equal(commonName))
			})
			It("should be a CA", func() {
				Expect(cert.IsCA).To(BeTrue())
			})
			It("should be valid for hosts", func() {
				for _, host := range hosts {
					Expect(cert.VerifyHostname(host)).To(BeNil())
				}
			})
			It("should be only valid from now", func() {
				Expect(cert.NotBefore).To(Equal(now()))
			})
		}

		Context("with an RSA cert", func() {
			BeforeEach(func() {
				algorithm = "rsa"
				size = "2048"
				lifespan = "365d"
				commonName = "test"
				hosts = []string{"localhost", "127.0.0.1"}
			})

			CommonBehavior()

			It("should be only valid for one year", func() {
				Expect(cert.NotAfter).To(Equal(now().Add(365 * 24 * time.Hour)))
			})
			It("should use the SHA256-RSA SignatureAlgorithm", func() {
				Expect(cert.SignatureAlgorithm.String()).To(Equal("SHA256-RSA"))
			})
			It("should use the RSA PublicKeyAlgorithm", func() {
				Expect(cert.PublicKeyAlgorithm.String()).To(Equal("RSA"))
			})
		})

		Context("with an RSA cert", func() {
			BeforeEach(func() {
				algorithm = "rsa"
				size = "2048"
				lifespan = "24h"
				commonName = "test"
				hosts = []string{"localhost", "127.0.0.1"}
			})

			CommonBehavior()

			It("should be only valid for one day", func() {
				Expect(cert.NotAfter).To(Equal(now().Add(24 * time.Hour)))
			})
			It("should use the SHA256-RSA SignatureAlgorithm", func() {
				Expect(cert.SignatureAlgorithm.String()).To(Equal("SHA256-RSA"))
			})
			It("should use the RSA PublicKeyAlgorithm", func() {
				Expect(cert.PublicKeyAlgorithm.String()).To(Equal("RSA"))
			})
		})

		Context("with an Ed25519 cert", func() {
			BeforeEach(func() {
				algorithm = "ed25519"
				size = "0"
				lifespan = "365d"
				commonName = "test"
				hosts = []string{"localhost", "127.0.0.1"}
			})

			CommonBehavior()

			It("should be only valid for one year", func() {
				Expect(cert.NotAfter).To(Equal(now().Add(365 * 24 * time.Hour)))
			})
			It("should use the Ed25519 SignatureAlgorithm", func() {
				Expect(cert.SignatureAlgorithm.String()).To(Equal("Ed25519"))
			})
			It("should use the Ed25519 PublicKeyAlgorithm", func() {
				Expect(cert.PublicKeyAlgorithm.String()).To(Equal("Ed25519"))
			})
		})

		Context("with an ECDSA P256 cert", func() {
			BeforeEach(func() {
				algorithm = "ecdsa"
				size = "256"
				lifespan = "365d"
				commonName = "test"
				hosts = []string{"localhost", "127.0.0.1"}
			})

			CommonBehavior()

			It("should be only valid for one year", func() {
				Expect(cert.NotAfter).To(Equal(now().Add(365 * 24 * time.Hour)))
			})
			It("should use the ECDSA-SHA256 SignatureAlgorithm", func() {
				Expect(cert.SignatureAlgorithm.String()).To(Equal("ECDSA-SHA256"))
			})
			It("should use the ECDSA PublicKeyAlgorithm", func() {
				Expect(cert.PublicKeyAlgorithm.String()).To(Equal("ECDSA"))
			})
		})

		Context("with an ECDSA P384 cert", func() {
			BeforeEach(func() {
				algorithm = "ecdsa"
				size = "384"
				lifespan = "365d"
				commonName = "test"
				hosts = []string{"localhost", "127.0.0.1"}
			})

			CommonBehavior()

			It("should be only valid for one year", func() {
				Expect(cert.NotAfter).To(Equal(now().Add(365 * 24 * time.Hour)))
			})
			It("should use the ECDSA-SHA384 SignatureAlgorithm", func() {
				Expect(cert.SignatureAlgorithm.String()).To(Equal("ECDSA-SHA384"))
			})
			It("should use the ECDSA PublicKeyAlgorithm", func() {
				Expect(cert.PublicKeyAlgorithm.String()).To(Equal("ECDSA"))
			})
		})

		Context("with an ECDSA P521 cert", func() {
			BeforeEach(func() {
				algorithm = "ecdsa"
				size = "521"
				lifespan = "365d"
				commonName = "test"
				hosts = []string{"localhost", "127.0.0.1"}
			})

			CommonBehavior()

			It("should be only valid for one year", func() {
				Expect(cert.NotAfter).To(Equal(now().Add(365 * 24 * time.Hour)))
			})
			It("should use the ECDSA-SHA512 SignatureAlgorithm", func() {
				Expect(cert.SignatureAlgorithm.String()).To(Equal("ECDSA-SHA512"))
			})
			It("should use the ECDSA PublicKeyAlgorithm", func() {
				Expect(cert.PublicKeyAlgorithm.String()).To(Equal("ECDSA"))
			})
		})
	})
})

func startServer(socket string, cert []byte, key []byte) (*http.Server, net.Listener, error) {
	c, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{c},
		// #nosec G402 -- only for testing
		InsecureSkipVerify: true,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	})

	// #nosec G112 -- only for testing
	server := &http.Server{
		Handler:   mux,
		TLSConfig: tlsConfig,
		ErrorLog:  log.New(io.Discard, "", 0),
	}

	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, nil, err
	}

	go func() {
		_ = server.ServeTLS(listener, "", "")
		defer func() { _ = listener.Close() }()
	}()

	return server, listener, nil
}

func httpsClient(socket string, verify bool) (string, error) {
	tr := &http.Transport{
		// #nosec G402 -- only for testing
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !verify},
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socket)
		},
	}

	client := &http.Client{Transport: tr}

	resp, err := client.Get("https://unix")
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func certClient(socket string, verify bool) ([]*x509.Certificate, error) {
	conf := &tls.Config{
		// #nosec G402 -- only for testing
		InsecureSkipVerify: !verify,
	}
	conn, err := tls.Dial("unix", socket, conf)

	if err != nil {
		return nil, err
	}

	defer func() { _ = conn.Close() }()

	certs := conn.ConnectionState().PeerCertificates

	return certs, nil
}

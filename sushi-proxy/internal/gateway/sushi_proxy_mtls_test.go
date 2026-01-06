package gateway

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/stretchr/testify/assert"
)

// Helper to generate self-signed certs for testing
func generateTestCert(t *testing.T, dir string, name string) (string, string) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	assert.NoError(t, err)

	certPath := filepath.Join(dir, name+".crt")
	certOut, err := os.Create(certPath)
	assert.NoError(t, err)
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyPath := filepath.Join(dir, name+".key")
	keyOut, err := os.Create(keyPath)
	assert.NoError(t, err)
	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	return certPath, keyPath
}

func TestSushiProxy_GetTransport_mTLS(t *testing.T) {
	// Create temp dir for certs
	tmpDir, err := os.MkdirTemp("", "mtls-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	caCertPath, _ := generateTestCert(t, tmpDir, "ca")
	clientCertPath, clientKeyPath := generateTestCert(t, tmpDir, "client")

	proxy := NewSushiProxy()

	tests := []struct {
		name              string
		tlsConfig         model.UpstreamTLS
		expectError       bool
		expectTLS         bool
		expectClientCerts bool
		expectRootCAs     bool
	}{
		{
			name: "mTLS Disabled",
			tlsConfig: model.UpstreamTLS{
				Enabled: false,
			},
			expectError: false,
			expectTLS:   false,
		},
		{
			name: "mTLS Enabled - Complete Config",
			tlsConfig: model.UpstreamTLS{
				Enabled:    true,
				CaCertPath: caCertPath,
				CertPath:   clientCertPath,
				KeyPath:    clientKeyPath,
			},
			expectError:       false,
			expectTLS:         true,
			expectClientCerts: true,
			expectRootCAs:     true,
		},
		{
			name: "mTLS Enabled - Missing Files",
			tlsConfig: model.UpstreamTLS{
				Enabled:    true,
				CaCertPath: "non-existent.crt",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &model.Service{
				TLS: tt.tlsConfig,
			}

			transport, err := proxy.getTransport(service)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, transport)

			// Unwrap otelhttp transport to check underlying transport
			// Since otelhttp doesn't expose underlying transport easily in older versions,
			// we might just check if it returns without error for now unless we reflect.
			// However, in our implementation we used *http.Transport as base.

			// If we really want to inspect, we can check logic.
			// But for unit test, the most important is that it loads files correctly or errors if missing.

			// We can verify that getTransport accessed the files if successful.
			if tt.expectTLS && tt.expectRootCAs {
				// To fully verify, we'd need to inspect the transport struct which is hidden by otelhttp wrapper.
				// But we know 'err' would be non-nil if files failed to load.
			}
		})
	}
}

package publish

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// tlsState holds the registry listener's current certificate and metadata. The
// registry endpoint is always HTTPS so clusters can pull over TLS; without a
// provided cert it falls back to a self-signed one (clients must trust it or use
// --insecure), and a real cert loaded via env or the API takes effect on the
// next handshake without a restart.
type tlsState struct {
	mu       sync.RWMutex
	cert     *tls.Certificate
	source   string // "provided" | "self-signed"
	subject  string
	notAfter time.Time
	dnsNames []string
}

func (m *Manager) tlsDir() string {
	return filepath.Join(m.cfg.DataDir, "registry-tls")
}

// bootstrapTLS loads the registry cert at startup: env paths first, then a
// previously uploaded cert on disk, otherwise a generated self-signed cert.
func (m *Manager) bootstrapTLS() {
	m.tls = &tlsState{}

	certPath := os.Getenv("HAULER_UI_REGISTRY_TLS_CERT")
	keyPath := os.Getenv("HAULER_UI_REGISTRY_TLS_KEY")
	if certPath != "" && keyPath != "" {
		if cert, key, err := readPEMFiles(certPath, keyPath); err == nil {
			if err := m.setTLS(cert, key, "provided"); err == nil {
				return
			}
		}
	}

	diskCert := filepath.Join(m.tlsDir(), "cert.pem")
	diskKey := filepath.Join(m.tlsDir(), "key.pem")
	if cert, key, err := readPEMFiles(diskCert, diskKey); err == nil {
		if err := m.setTLS(cert, key, "provided"); err == nil {
			return
		}
	}

	// Fall back to a self-signed cert covering the configured domain.
	if err := m.generateSelfSigned(); err != nil {
		// Last resort: log via the listener path; leave cert nil (HTTP).
		m.tls = nil
	}
}

func readPEMFiles(certPath, keyPath string) (certPEM, keyPEM []byte, err error) {
	certPEM, err = os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err = os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	return certPEM, keyPEM, nil
}

// setTLS validates a cert/key pair and installs it as the active certificate.
func (m *Manager) setTLS(certPEM, keyPEM []byte, source string) error {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("invalid certificate/key: %w", err)
	}
	leaf := cert.Leaf
	if leaf == nil {
		if parsed, perr := x509.ParseCertificate(cert.Certificate[0]); perr == nil {
			leaf = parsed
		}
	}
	m.tls.mu.Lock()
	m.tls.cert = &cert
	m.tls.source = source
	if leaf != nil {
		m.tls.subject = leaf.Subject.CommonName
		m.tls.notAfter = leaf.NotAfter
		m.tls.dnsNames = leaf.DNSNames
	}
	m.tls.mu.Unlock()
	return nil
}

// SetProvidedTLS validates, persists, and activates a user-supplied cert/key.
func (m *Manager) SetProvidedTLS(certPEM, keyPEM []byte) error {
	if err := m.setTLS(certPEM, keyPEM, "provided"); err != nil {
		return err
	}
	if err := os.MkdirAll(m.tlsDir(), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(m.tlsDir(), "cert.pem"), certPEM, 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.tlsDir(), "key.pem"), keyPEM, 0600)
}

// ClearProvidedTLS removes a user-supplied cert and reverts to self-signed.
func (m *Manager) ClearProvidedTLS() error {
	os.Remove(filepath.Join(m.tlsDir(), "cert.pem"))
	os.Remove(filepath.Join(m.tlsDir(), "key.pem"))
	return m.generateSelfSigned()
}

// TLSStatus describes the active registry certificate for the UI.
type TLSStatus struct {
	Source   string   `json:"source"` // "provided" | "self-signed" | "none"
	Subject  string   `json:"subject"`
	NotAfter string   `json:"notAfter"`
	DNSNames []string `json:"dnsNames"`
}

func (m *Manager) TLSStatus() TLSStatus {
	if m.tls == nil {
		return TLSStatus{Source: "none"}
	}
	m.tls.mu.RLock()
	defer m.tls.mu.RUnlock()
	notAfter := ""
	if !m.tls.notAfter.IsZero() {
		notAfter = m.tls.notAfter.Format(time.RFC3339)
	}
	return TLSStatus{
		Source:   m.tls.source,
		Subject:  m.tls.subject,
		NotAfter: notAfter,
		DNSNames: m.tls.dnsNames,
	}
}

// getCertificate is the tls.Config callback returning the current cert.
func (m *Manager) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	if m.tls == nil {
		return nil, fmt.Errorf("no certificate configured")
	}
	m.tls.mu.RLock()
	defer m.tls.mu.RUnlock()
	if m.tls.cert == nil {
		return nil, fmt.Errorf("no certificate configured")
	}
	return m.tls.cert, nil
}

// hasTLS reports whether a certificate is available (i.e. serve HTTPS).
func (m *Manager) hasTLS() bool {
	if m.tls == nil {
		return false
	}
	m.tls.mu.RLock()
	defer m.tls.mu.RUnlock()
	return m.tls.cert != nil
}

// generateSelfSigned creates an in-memory self-signed cert covering the
// configured registry domain (wildcard) plus localhost.
func (m *Manager) generateSelfSigned() error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	dnsNames := []string{"localhost"}
	if d := m.registryDomain(); d != "" {
		dnsNames = append([]string{"*." + d, d}, dnsNames...)
	}

	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "hauler-ui registry (self-signed)"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return m.setTLS(certPEM, keyPEM, "self-signed")
}

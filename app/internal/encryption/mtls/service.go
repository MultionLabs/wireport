package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"time"
)

type Options struct {
	CommonName  string
	Expiry      time.Duration
	DNSNames    []string // Optional, used for server certs
	IPAddresses []string // Optional, used for server certs
}

type PEMBundle struct {
	CertPEM string `json:"cert_pem"`
	KeyPEM  string `json:"key_pem"`
}

type FullHostBundle struct {
	RootCA    PEMBundle            `json:"root_ca"`
	Server    PEMBundle            `json:"server"`
	Clients   map[string]PEMBundle `json:"clients"`
	Generated time.Time            `json:"generated_at"`
}

type FullClientBundle struct {
	RootCA    PEMBundle `json:"root_ca"`
	Client    PEMBundle `json:"client"`
	Generated time.Time `json:"generated_at"`
}

// Generate creates a full mTLS bundle with root CA and server cert
func Generate(serverOpt Options, rootExpiry time.Duration) (*FullHostBundle, error) {
	// Root CA
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	if err != nil {
		return nil, err
	}

	rootTpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "wireport host Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(rootExpiry),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	rootCert, err := x509.CreateCertificate(rand.Reader, rootTpl, rootTpl, &rootKey.PublicKey, rootKey)

	if err != nil {
		return nil, err
	}

	rootCertParsed, err := x509.ParseCertificate(rootCert)

	if err != nil {
		return nil, err
	}

	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootCert})
	rootKeyBytes, err := x509.MarshalECPrivateKey(rootKey)

	if err != nil {
		return nil, err
	}
	rootKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: rootKeyBytes,
	})

	serverCert, serverKey, err := createSignedCert(serverOpt, rootCertParsed, rootKey, true)

	if err != nil {
		return nil, err
	}

	bundle := &FullHostBundle{
		RootCA: PEMBundle{
			CertPEM: string(rootPEM),
			KeyPEM:  string(rootKeyPEM),
		},
		Server: PEMBundle{
			CertPEM: string(serverCert),
			KeyPEM:  string(serverKey),
		},
		Clients:   map[string]PEMBundle{},
		Generated: time.Now(),
	}

	return bundle, nil
}

func createSignedCert(opt Options, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, isServer bool) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName: opt.CommonName,
		},
		NotBefore:   now,
		NotAfter:    now.Add(opt.Expiry),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// If it's a server cert, add server auth
	if isServer {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

		if len(opt.DNSNames) > 0 {
			tpl.DNSNames = opt.DNSNames
		}

		if len(opt.IPAddresses) > 0 {
			tpl.IPAddresses = make([]net.IP, 0, len(opt.IPAddresses))
			for _, ipStr := range opt.IPAddresses {
				if ip := net.ParseIP(ipStr); ip != nil {
					tpl.IPAddresses = append(tpl.IPAddresses, ip)
				}
			}
		}
	}

	cert, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &key.PublicKey, caKey)

	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	keyBytes, err := x509.MarshalECPrivateKey(key)

	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	return certPEM, keyPEM, nil
}

// AddClient adds a new client cert to an existing bundle
func (b *FullHostBundle) AddClient(opt Options) error {
	if b.Clients == nil {
		b.Clients = make(map[string]PEMBundle)
	}

	caCertBlock, _ := pem.Decode([]byte(b.RootCA.CertPEM))
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)

	if err != nil {
		return err
	}

	caKeyBlock, _ := pem.Decode([]byte(b.RootCA.KeyPEM))
	caKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)

	if err != nil {
		return err
	}

	certPEM, keyPEM, err := createSignedCert(opt, caCert, caKey, false)

	if err != nil {
		return err
	}

	b.Clients[opt.CommonName] = PEMBundle{CertPEM: string(certPEM), KeyPEM: string(keyPEM)}

	return nil
}

func (b *FullHostBundle) RemoveClient(clientName string) error {
	delete(b.Clients, clientName)

	return nil
}

// TLSConfigs returns tls.Config for server and client
func (b *FullHostBundle) TLSConfigs(clientName string) (*tls.Config, *tls.Config, error) {
	if b.Server.KeyPEM == "" || b.Server.CertPEM == "" || b.RootCA.CertPEM == "" {
		return nil, nil, errors.New("server key, cert or root CA cert is empty")
	}

	serverCert, err := tls.X509KeyPair([]byte(b.Server.CertPEM), []byte(b.Server.KeyPEM))

	if err != nil {
		return nil, nil, err
	}

	clientData, ok := b.Clients[clientName]

	if !ok {
		return nil, nil, errors.New("client not found")
	}

	clientCert, err := tls.X509KeyPair([]byte(clientData.CertPEM), []byte(clientData.KeyPEM))

	if err != nil {
		return nil, nil, err
	}

	rootCAPool := x509.NewCertPool()
	rootCAPool.AppendCertsFromPEM([]byte(b.RootCA.CertPEM))

	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    rootCAPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	clientTLS := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      rootCAPool,
	}

	return serverTLS, clientTLS, nil
}

// PublicOnly returns a copy of the bundle without any private keys
func (b *FullHostBundle) PublicOnly() *FullHostBundle {
	clients := make(map[string]PEMBundle)

	for k, v := range b.Clients {
		clients[k] = PEMBundle{CertPEM: v.CertPEM}
	}

	return &FullHostBundle{
		RootCA:    PEMBundle{CertPEM: b.RootCA.CertPEM},
		Server:    PEMBundle{CertPEM: b.Server.CertPEM},
		Clients:   clients,
		Generated: b.Generated,
	}
}

// GetClientBundlePublic returns a client bundle with Root CA certificate only (no private key)
func (b *FullHostBundle) GetClientBundlePublic(clientName string) (*FullClientBundle, error) {
	clientData, ok := b.Clients[clientName]

	if !ok {
		return nil, errors.New("client not found")
	}

	return &FullClientBundle{
		RootCA: PEMBundle{
			CertPEM: b.RootCA.CertPEM, // Only certificate, no private key
		},
		Client:    clientData, // Full client cert + key (client needs this)
		Generated: b.Generated,
	}, nil
}

// GetServerTLSConfig returns tls.Config for server only (for mTLS server setup)
func (b *FullHostBundle) GetServerTLSConfig() (*tls.Config, error) {
	if b.Server.KeyPEM == "" || b.Server.CertPEM == "" || b.RootCA.CertPEM == "" {
		return nil, errors.New("server key, cert or root CA cert is empty")
	}

	serverCert, err := tls.X509KeyPair([]byte(b.Server.CertPEM), []byte(b.Server.KeyPEM))

	if err != nil {
		return nil, err
	}

	rootCAPool := x509.NewCertPool()
	rootCAPool.AppendCertsFromPEM([]byte(b.RootCA.CertPEM))

	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    rootCAPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	return serverTLS, nil
}

// GetClientTLSConfig returns tls.Config for client only (for mTLS client setup)
func (b *FullClientBundle) GetClientTLSConfig() (*tls.Config, error) {
	if b.Client.KeyPEM == "" || b.Client.CertPEM == "" || b.RootCA.CertPEM == "" {
		return nil, errors.New("client key, cert or root CA cert is empty")
	}

	clientCert, err := tls.X509KeyPair([]byte(b.Client.CertPEM), []byte(b.Client.KeyPEM))

	if err != nil {
		return nil, err
	}

	rootCAPool := x509.NewCertPool()
	rootCAPool.AppendCertsFromPEM([]byte(b.RootCA.CertPEM))

	clientTLS := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      rootCAPool,
	}

	return clientTLS, nil
}

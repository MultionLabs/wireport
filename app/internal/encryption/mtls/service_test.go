package mtls

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSerialization(t *testing.T) {
	serverOpts := Options{CommonName: "localhost", Expiry: time.Hour, DNSNames: []string{"localhost"}}
	clientOpts := Options{CommonName: "client1", Expiry: time.Hour}
	bundle, err := Generate(serverOpts, time.Hour)

	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if err := bundle.AddClient(clientOpts); err != nil {
		t.Fatalf("add client1 failed: %v", err)
	}

	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	var copy FullHostBundle
	err = json.Unmarshal(data, &copy)
	if err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	if copy.RootCA.CertPEM != bundle.RootCA.CertPEM {
		t.Fatal("root CA cert mismatch")
	}
}

func TestMTLSCommunication(t *testing.T) {
	serverOpts := Options{
		CommonName:  "localhost",
		Expiry:      time.Hour,
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1"},
	}
	client1Opts := Options{CommonName: "client1", Expiry: time.Hour}
	client2Opts := Options{CommonName: "client2", Expiry: time.Hour}
	bundle, err := Generate(serverOpts, time.Hour)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if err := bundle.AddClient(client1Opts); err != nil {
		t.Fatalf("add client1 failed: %v", err)
	}

	if err := bundle.AddClient(client2Opts); err != nil {
		t.Fatalf("add client2 failed: %v", err)
	}

	serverTLS, _, err := bundle.TLSConfigs("client1") // temporary to get server config
	if err != nil {
		t.Fatalf("TLS config: %v", err)
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cert := r.TLS.PeerCertificates[0]
		fmt.Fprintf(w, "hello, %s", cert.Subject.CommonName)
	})

	ts := httptest.NewUnstartedServer(h)
	ts.TLS = serverTLS
	ts.StartTLS()
	defer ts.Close()

	for _, clientName := range []string{"client1", "client2"} {
		_, clientTLS, err := bundle.TLSConfigs(clientName)
		if err != nil {
			t.Fatalf("TLS config client %s: %v", clientName, err)
		}
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS}}
		resp, err := client.Get(ts.URL)
		if err != nil {
			t.Fatalf("client %s failed: %v", clientName, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !bytes.Contains(body, []byte(clientName)) {
			t.Fatalf("expected response to contain %q, got %q", clientName, string(body))
		}
		log.Printf("%s OK: %q", clientName, string(body))
	}
}

func TestSerializationAndHTTPSCommunication(t *testing.T) {
	// Generate original bundle
	serverOpts := Options{
		CommonName:  "localhost",
		Expiry:      time.Hour,
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1"},
	}
	client1Opts := Options{CommonName: "client1", Expiry: time.Hour}
	client2Opts := Options{CommonName: "client2", Expiry: time.Hour}

	originalBundle, err := Generate(serverOpts, time.Hour)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if err := originalBundle.AddClient(client1Opts); err != nil {
		t.Fatalf("add client1 failed: %v", err)
	}

	// Serialize the bundle (only with client1)
	data, err := json.Marshal(originalBundle)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	// Unmarshal to get a copy
	var unmarshaledBundle FullHostBundle
	err = json.Unmarshal(data, &unmarshaledBundle)
	if err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	// Add second client after unmarshalling
	if err := unmarshaledBundle.AddClient(client2Opts); err != nil {
		t.Fatalf("add client2 to unmarshaled bundle failed: %v", err)
	}

	// Verify serialization worked correctly
	if unmarshaledBundle.RootCA.CertPEM != originalBundle.RootCA.CertPEM {
		t.Fatal("root CA cert mismatch after serialization")
	}

	// Use the unmarshaled bundle for HTTPS communication test
	serverTLS, _, err := unmarshaledBundle.TLSConfigs("client1")
	if err != nil {
		t.Fatalf("TLS config from unmarshaled bundle: %v", err)
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cert := r.TLS.PeerCertificates[0]
		fmt.Fprintf(w, "hello from unmarshaled bundle, %s", cert.Subject.CommonName)
	})

	ts := httptest.NewUnstartedServer(h)
	ts.TLS = serverTLS
	ts.StartTLS()
	defer ts.Close()

	// Test both clients with the unmarshaled bundle
	for _, clientName := range []string{"client1", "client2"} {
		_, clientTLS, err := unmarshaledBundle.TLSConfigs(clientName)
		if err != nil {
			t.Fatalf("TLS config client %s from unmarshaled bundle: %v", clientName, err)
		}
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS}}
		resp, err := client.Get(ts.URL)
		if err != nil {
			t.Fatalf("client %s failed with unmarshaled bundle: %v", clientName, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !bytes.Contains(body, []byte(clientName)) {
			t.Fatalf("expected response to contain %q, got %q", clientName, string(body))
		}
		log.Printf("%s OK with unmarshaled bundle: %q", clientName, string(body))
	}
}

func TestServerAndClientTLSConfigs(t *testing.T) {
	serverOpts := Options{
		CommonName:  "localhost",
		Expiry:      time.Hour,
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1"},
	}
	clientOpts := Options{CommonName: "client1", Expiry: time.Hour}

	bundle, err := Generate(serverOpts, time.Hour)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if err := bundle.AddClient(clientOpts); err != nil {
		t.Fatalf("add client failed: %v", err)
	}

	// Test server TLS config
	serverTLS, err := bundle.GetServerTLSConfig()
	if err != nil {
		t.Fatalf("GetServerTLSConfig failed: %v", err)
	}
	if serverTLS == nil {
		t.Fatal("server TLS config is nil")
	}
	if len(serverTLS.Certificates) != 1 {
		t.Fatal("server TLS config should have exactly one certificate")
	}
	if serverTLS.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatal("server TLS config should require client certificates")
	}

	// Test client TLS config
	clientBundle, err := bundle.GetClientBundlePublic("client1")
	if err != nil {
		t.Fatalf("GetClientBundlePublic failed: %v", err)
	}

	clientTLS, err := clientBundle.GetClientTLSConfig()
	if err != nil {
		t.Fatalf("GetClientTLSConfig failed: %v", err)
	}
	if clientTLS == nil {
		t.Fatal("client TLS config is nil")
	}
	if len(clientTLS.Certificates) != 1 {
		t.Fatal("client TLS config should have exactly one certificate")
	}
	if clientTLS.RootCAs == nil {
		t.Fatal("client TLS config should have root CAs")
	}
}

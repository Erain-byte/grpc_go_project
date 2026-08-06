package grpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRegistry struct {
	mu      sync.Mutex
	entries []*api.ServiceEntry
	err     error
	names   []string
}

func (f *fakeRegistry) DiscoverGRPCService(_ context.Context, name string) ([]*api.ServiceEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.names = append(f.names, name)
	return f.entries, f.err
}

func TestNewClientManagerDoesNotMutateConfig(t *testing.T) {
	cfg := &GrpcConfig{UseTLS: true}
	_ = NewClientManager(&fakeRegistry{}, cfg)
	if cfg.CertFile != "" || cfg.KeyFile != "" || cfg.CaFile != "" {
		t.Fatalf("NewClientManager mutated caller config: %+v", cfg)
	}
}

func TestCreateTransportCredentialsRejectsPartialClientCertificate(t *testing.T) {
	manager := NewClientManager(&fakeRegistry{}, &GrpcConfig{
		UseTLS:   true,
		CertFile: "client.crt",
	})
	_, err := manager.createTransportCredentials()
	if err == nil {
		t.Fatal("createTransportCredentials() error = nil, want partial certificate error")
	}
}

func TestGetClientReusesConnectionConcurrently(t *testing.T) {
	manager := NewClientManager(&fakeRegistry{}, &GrpcConfig{})
	t.Cleanup(manager.Close)

	const callers = 20
	connections := make(chan any, callers)
	errors := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := manager.GetClient(context.Background(), "admin-service")
			if err != nil {
				errors <- err
				return
			}
			connections <- conn
		}()
	}
	wg.Wait()
	close(connections)
	close(errors)
	for err := range errors {
		t.Fatalf("GetClient() error = %v", err)
	}

	var first any
	for conn := range connections {
		if first == nil {
			first = conn
			continue
		}
		if conn != first {
			t.Fatal("GetClient() created more than one connection for the same service")
		}
	}
}

func TestGetClientAfterClose(t *testing.T) {
	manager := NewClientManager(&fakeRegistry{}, &GrpcConfig{})
	manager.Close()
	_, err := manager.GetClient(context.Background(), "admin-service")
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("GetClient() code = %v, want %v", status.Code(err), codes.Unavailable)
	}
}

func TestCheckCertificateExpiry(t *testing.T) {
	tests := []struct {
		name     string
		notAfter time.Time
		wantCode codes.Code
	}{
		{name: "valid", notAfter: time.Now().Add(30 * 24 * time.Hour), wantCode: codes.OK},
		{name: "expiring", notAfter: time.Now().Add(24 * time.Hour), wantCode: codes.InvalidArgument},
		{name: "expired", notAfter: time.Now().Add(-time.Hour), wantCode: codes.InvalidArgument},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			certFile, keyFile := writeTestCertificate(t, test.notAfter)
			manager := NewClientManager(&fakeRegistry{}, &GrpcConfig{
				UseTLS:   true,
				CertFile: certFile,
				KeyFile:  keyFile,
			})
			err := manager.CheckCertificateExpiry(context.Background(), "")
			if status.Code(err) != test.wantCode {
				t.Fatalf("CheckCertificateExpiry() code = %v, want %v (err=%v)", status.Code(err), test.wantCode, err)
			}
		})
	}
}

func writeTestCertificate(t *testing.T, notAfter time.Time) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	dir := t.TempDir()
	certFile := filepath.Join(dir, "client.crt")
	keyFile := filepath.Join(dir, "client.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile
}

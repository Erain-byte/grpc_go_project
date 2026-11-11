// Command generate-mtls creates local development mTLS identities.
// Generated private keys and certificates live under certs/mtls and are ignored by Git.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type identity struct {
	directory string
	filename  string
	name      string
	dnsNames  []string
	usages    []x509.ExtKeyUsage
}

func main() {
	root := filepath.FromSlash("certs/mtls")
	now := time.Now().UTC()

	caKey := mustKey()
	caTemplate := &x509.Certificate{
		SerialNumber:          mustSerial(),
		Subject:               pkix.Name{CommonName: "grpc-go-project internal development CA"},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	must(err)
	writeCertificate(filepath.Join(root, "ca", "ca.crt"), caDER)
	writeKey(filepath.Join(root, "ca", "ca.key"), caKey)

	identities := []identity{
		{directory: "gateway", filename: "service", name: "gateway-service", dnsNames: []string{"gateway-service", "localhost", "host.docker.internal"}, usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}},
		{directory: "consul", filename: "client", name: "consul-health-check", usages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}},
		{directory: "admin", filename: "service", name: "admin-service", dnsNames: []string{"admin-service", "localhost", "host.docker.internal"}, usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}},
		{directory: "user", filename: "service", name: "user-service", dnsNames: []string{"user-service", "localhost", "host.docker.internal"}, usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}},
		{directory: "web3", filename: "server", name: "web3-service", dnsNames: []string{"web3-service", "localhost", "host.docker.internal"}, usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}},
	}
	for _, item := range identities {
		key := mustKey()
		template := &x509.Certificate{
			SerialNumber: mustSerial(),
			Subject:      pkix.Name{CommonName: item.name},
			NotBefore:    now.Add(-5 * time.Minute),
			NotAfter:     now.AddDate(1, 0, 0),
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:  item.usages,
			DNSNames:     item.dnsNames,
			IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		}
		der, createErr := x509.CreateCertificate(rand.Reader, template, caTemplate, &key.PublicKey, caKey)
		must(createErr)
		base := filepath.Join(root, item.directory, item.filename)
		writeCertificate(base+".crt", der)
		writeKey(base+".key", key)
	}

	fmt.Printf("generated local mTLS material under %s\n", root)
}

func mustKey() *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(err)
	return key
}

func mustSerial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	must(err)
	return serial
}

func writeCertificate(path string, der []byte) {
	must(os.MkdirAll(filepath.Dir(path), 0o700))
	must(os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
}

func writeKey(path string, key *ecdsa.PrivateKey) {
	der, err := x509.MarshalECPrivateKey(key)
	must(err)
	must(os.MkdirAll(filepath.Dir(path), 0o700))
	must(os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

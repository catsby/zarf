package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// Based off of https://github.com/dmcgowan/quicktls/blob/master/main.go

// Use 2048 because we are aiming for low-resource / max-compatability
const rsaBits = 2048
const org = "Zarf Utility Cluster"

// 13 months is the max length allowed by browsers
const validFor = time.Hour * 24 * 375

// Very limited special chars for git / basic auth 
// https://owasp.org/www-community/password-special-characters has complete list of safe chars
const randomStringChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!~-"

func RandomString(length int) string {
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		logrus.Fatal("unable to generate a random secret")
	}

	for i, b := range bytes {
		bytes[i] = randomStringChars[b%byte(len(randomStringChars))]
	}

	return string(bytes)
}

// GeneratePKI create a CA and signed server keypair
func GeneratePKI(host string) {
	directory := AssetPath("certs")

	CreateDirectory(directory, 0700)
	caFile := filepath.Join(directory, "zarf-ca.pem")
	ca, caKey, err := generateCA(caFile, validFor)
	if err != nil {
		logrus.Fatal(err)
	}

	hostCert := filepath.Join(directory, "zarf-server.crt")
	hostKey := filepath.Join(directory, "zarf-server.key")
	if err := generateCert(host, hostCert, hostKey, ca, caKey, validFor); err != nil {
		logrus.Fatal(err)
	}

	publicKeyBlock := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	}

	publicKeyPem := string(pem.EncodeToMemory(&publicKeyBlock))

	// Push the certs to the cluster
	ExecCommand([]string{}, "/usr/local/bin/kubectl", "-n", "kube-system", "delete", "secret", "tls-pem", "--ignore-not-found")
	ExecCommand([]string{}, "/usr/local/bin/kubectl", "-n", "kube-system", "create", "secret", "tls", "tls-pem", "--cert="+directory+"/zarf-server.crt", "--key="+directory+"/zarf-server.key")

	fmt.Println("Ephemeral CA below and saved to " + caFile + "\n")
	fmt.Println(publicKeyPem)
}

// newCertificate creates a new template
func newCertificate(validFor time.Duration) *x509.Certificate {
	notBefore := time.Now()
	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		logrus.Fatalf("failed to generate serial number: %s", err)
	}

	return &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{org},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
}

// newPrivateKey creates a new private key
func newPrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, rsaBits)
}

// generateCA creates a new CA certificate, saves the certificate
// and returns the x509 certificate and crypto private key. This
// private key should never be saved to disk, but rather used to
// immediately generate further certificates.
func generateCA(caFile string, validFor time.Duration) (*x509.Certificate, *rsa.PrivateKey, error) {
	template := newCertificate(validFor)
	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign
	template.Subject.CommonName = "Zarf Private Certificate Authority"

	priv, err := newPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, priv.Public(), priv)
	if err != nil {
		return nil, nil, err
	}

	ca, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, err
	}

	certOut, err := os.Create(caFile)
	if err != nil {
		return nil, nil, err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, nil, err
	}

	return ca, priv, nil
}

// generateCert generates a new certificate for the given host using the
// provided certificate authority. The cert and key files are stored in the
// the provided files.
func generateCert(host string, certFile string, keyFile string, ca *x509.Certificate, caKey *rsa.PrivateKey, validFor time.Duration) error {
	template := newCertificate(validFor)

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		// Add localhost to make things cleaner
		template.DNSNames = append(template.DNSNames, host, "localhost", "*.localhost")
		if template.Subject.CommonName == "" {
			template.Subject.CommonName = host
		}
	}

	priv, err := newPrivateKey()
	if err != nil {
		return err
	}

	return generateFromTemplate(certFile, keyFile, template, ca, priv, caKey)
}

// generateFromTemplate generates a certificate from the given template and signed by
// the given parent, storing the results in a certificate and key file.
func generateFromTemplate(certFile, keyFile string, template, parent *x509.Certificate, key *rsa.PrivateKey, parentKey *rsa.PrivateKey) error {
	derBytes, err := x509.CreateCertificate(rand.Reader, template, parent, key.Public(), parentKey)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	return savePrivateKey(key, keyFile)
}

// savePrivateKey saves the private key to a PEM file
func savePrivateKey(key *rsa.PrivateKey, keyFile string) error {
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})

	return nil
}
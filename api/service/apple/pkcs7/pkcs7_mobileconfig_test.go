//go:build !pkcs7_legacy

package pkcs7

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"
)

func TestSignedData_SignAndVerify_TableDriven(t *testing.T) {
	content := []byte("hello")

	rsaCert, rsaKey := mustSelfSignedRSA(t)
	ecdsaCert, ecdsaKey := mustSelfSignedECDSA(t)

	tests := []struct {
		name       string
		cert       *x509.Certificate
		key        crypto.PrivateKey
		detach     bool
		addExtra   bool
		wantSigner *x509.Certificate
	}{
		{name: "rsa/attached", cert: rsaCert, key: rsaKey, detach: false, addExtra: false, wantSigner: rsaCert},
		{name: "rsa/detached", cert: rsaCert, key: rsaKey, detach: true, addExtra: false, wantSigner: rsaCert},
		{name: "ecdsa/attached", cert: ecdsaCert, key: ecdsaKey, detach: false, addExtra: true, wantSigner: ecdsaCert},
		{name: "ecdsa/detached", cert: ecdsaCert, key: ecdsaKey, detach: true, addExtra: true, wantSigner: ecdsaCert},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd, err := NewSignedData(content)
			if err != nil {
				t.Fatalf("NewSignedData: %v", err)
			}

			cfg := SignerInfoConfig{}
			if tt.addExtra {
				cfg.ExtraSignedAttributes = []Attribute{{Type: asn1.ObjectIdentifier{1, 2, 3, 4}, Value: "extra"}}
			}

			if err := sd.AddSigner(tt.cert, tt.key, cfg); err != nil {
				t.Fatalf("AddSigner: %v", err)
			}
			if tt.detach {
				sd.Detach()
			}
			signed, err := sd.Finish()
			if err != nil {
				t.Fatalf("Finish: %v", err)
			}

			p7, err := Parse(signed)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if tt.detach {
				p7.Content = content
			}
			if !bytes.Equal(p7.Content, content) {
				t.Fatalf("content mismatch")
			}

			if signer := p7.GetOnlySigner(); signer == nil {
				t.Fatalf("GetOnlySigner: got nil")
			} else if !bytes.Equal(signer.Raw, tt.wantSigner.Raw) {
				t.Fatalf("GetOnlySigner: unexpected signer")
			}

			// Verify signature integrity and authenticatedAttributes messageDigest.
			if err := p7.Verify(); err != nil {
				t.Fatalf("Verify: %v", err)
			}

			// Also ensure we can pull common signed attributes.
			var ct asn1.ObjectIdentifier
			if err := p7.UnmarshalSignedAttribute(oidAttributeContentType, &ct); err != nil {
				t.Fatalf("UnmarshalSignedAttribute(contentType): %v", err)
			}
			if !ct.Equal(oidData) {
				t.Fatalf("contentType mismatch: got %v want %v", ct, oidData)
			}

			var md []byte
			if err := p7.UnmarshalSignedAttribute(oidAttributeMessageDigest, &md); err != nil {
				t.Fatalf("UnmarshalSignedAttribute(messageDigest): %v", err)
			}
			h := crypto.SHA256.New()
			h.Write(content)
			wantMD := h.Sum(nil)
			if !bytes.Equal(md, wantMD) {
				t.Fatalf("messageDigest mismatch")
			}
		})
	}
}

func TestPKCS7_Verify_TamperedContentFails(t *testing.T) {
	cert, key := mustSelfSignedRSA(t)
	content := []byte("hello")

	sd, err := NewSignedData(content)
	if err != nil {
		t.Fatalf("NewSignedData: %v", err)
	}
	if err := sd.AddSigner(cert, key, SignerInfoConfig{}); err != nil {
		t.Fatalf("AddSigner: %v", err)
	}
	signed, err := sd.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	p7, err := Parse(signed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	p7.Content = []byte("goodbye")
	if err := p7.Verify(); err == nil {
		t.Fatalf("Verify: expected error")
	}
}

func TestPKCS7_Verify_UnsupportedDigestOID(t *testing.T) {
	cert, key := mustSelfSignedRSA(t)
	content := []byte("hello")

	sd, err := NewSignedData(content)
	if err != nil {
		t.Fatalf("NewSignedData: %v", err)
	}
	if err := sd.AddSigner(cert, key, SignerInfoConfig{}); err != nil {
		t.Fatalf("AddSigner: %v", err)
	}
	signed, err := sd.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	p7, err := Parse(signed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Force an unsupported digest algorithm OID.
	p7.Signers[0].DigestAlgorithm.Algorithm = asn1.ObjectIdentifier{1, 2, 3}
	if err := p7.Verify(); err == nil {
		t.Fatalf("Verify: expected error")
	}
}

func TestPKCS7_GetOnlySigner_MultipleSignersNil(t *testing.T) {
	cert, key := mustSelfSignedRSA(t)
	content := []byte("hello")

	sd, err := NewSignedData(content)
	if err != nil {
		t.Fatalf("NewSignedData: %v", err)
	}
	if err := sd.AddSigner(cert, key, SignerInfoConfig{}); err != nil {
		t.Fatalf("AddSigner(1): %v", err)
	}
	if err := sd.AddSigner(cert, key, SignerInfoConfig{}); err != nil {
		t.Fatalf("AddSigner(2): %v", err)
	}
	signed, err := sd.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	p7, err := Parse(signed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if signer := p7.GetOnlySigner(); signer != nil {
		t.Fatalf("GetOnlySigner: expected nil")
	}
}

func mustSelfSignedRSA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	cert, err := selfSignedCert(priv.Public(), priv)
	if err != nil {
		t.Fatalf("selfSignedCert: %v", err)
	}
	return cert, priv
}

func mustSelfSignedECDSA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	cert, err := selfSignedCert(priv.Public(), priv)
	if err != nil {
		t.Fatalf("selfSignedCert: %v", err)
	}
	return cert, priv
}

func selfSignedCert(pub any, signer any) (*x509.Certificate, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 62)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	tpl := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "pkcs7-test",
			Organization: []string{"Acme"},
		},
		NotBefore: time.Now().Add(-1 * time.Minute),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, pub, signer)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}

//go:build !pkcs7_legacy

package pkcs7

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"
)

// This file is the mobileconfig-focused subset of PKCS#7:
// - Signing: SHA-256 + RSA/ECDSA
// - Verification: signature + messageDigest only (no chain/time validation)
// - No CMS encryption (no DES, no SHA-1)

// PKCS7 Represents a PKCS7 structure.
// NOTE: Verification here checks signature integrity only.

type PKCS7 struct {
	Content      []byte
	Certificates []*x509.Certificate
	CRLs         []*x509.RevocationList
	Signers      []signerInfo
	raw          interface{}
}

type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,optional,tag:0"`
}

// ErrUnsupportedContentType is returned when a PKCS7 content is not supported.
var ErrUnsupportedContentType = errors.New("pkcs7: cannot parse data: unimplemented content type")

type unsignedData []byte

var (
	oidData                   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSignedData             = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidAttributeContentType   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidAttributeMessageDigest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidAttributeSigningTime   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}

	oidEncryptionAlgorithmRSA = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
)

type signedData struct {
	Version                    int                        `asn1:"default:1"`
	DigestAlgorithmIdentifiers []pkix.AlgorithmIdentifier `asn1:"set"`
	ContentInfo                contentInfo
	Certificates               rawCertificates `asn1:"optional,tag:0"`
	CRLs                       []asn1.RawValue `asn1:"optional,tag:1"`
	SignerInfos                []signerInfo    `asn1:"set"`
}

type rawCertificates struct {
	Raw asn1.RawContent
}

type attribute struct {
	Type  asn1.ObjectIdentifier
	Value asn1.RawValue `asn1:"set"`
}

type issuerAndSerial struct {
	IssuerName   asn1.RawValue
	SerialNumber *big.Int
}

type MessageDigestMismatchError struct {
	ExpectedDigest []byte
	ActualDigest   []byte
}

func (err *MessageDigestMismatchError) Error() string {
	return fmt.Sprintf("pkcs7: Message digest mismatch\n\tExpected: %X\n\tActual  : %X", err.ExpectedDigest, err.ActualDigest)
}

type signerInfo struct {
	Version                   int `asn1:"default:1"`
	IssuerAndSerialNumber     issuerAndSerial
	DigestAlgorithm           pkix.AlgorithmIdentifier
	AuthenticatedAttributes   []attribute `asn1:"optional,tag:0"`
	DigestEncryptionAlgorithm pkix.AlgorithmIdentifier
	EncryptedDigest           []byte
	UnauthenticatedAttributes []attribute `asn1:"optional,tag:1"`
}

// Parse decodes a DER encoded PKCS7 package.
func Parse(data []byte) (p7 *PKCS7, err error) {
	if len(data) == 0 {
		return nil, errors.New("pkcs7: input data is empty")
	}
	var info contentInfo
	der, err := ber2der(data)
	if err != nil {
		return nil, err
	}
	rest, err := asn1.Unmarshal(der, &info)
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, asn1.SyntaxError{Msg: "trailing data"}
	}

	if info.ContentType.Equal(oidSignedData) {
		return parseSignedData(info.Content.Bytes)
	}
	return nil, ErrUnsupportedContentType
}

func parseSignedData(data []byte) (*PKCS7, error) {
	var sd signedData
	if _, err := asn1.Unmarshal(data, &sd); err != nil {
		return nil, err
	}
	certs, err := sd.Certificates.Parse()
	if err != nil {
		return nil, err
	}

	crls, err := parseRevocationLists(sd.CRLs)
	if err != nil {
		return nil, err
	}

	var compound asn1.RawValue
	var content unsignedData

	if len(sd.ContentInfo.Content.Bytes) > 0 {
		if _, err := asn1.Unmarshal(sd.ContentInfo.Content.Bytes, &compound); err != nil {
			return nil, err
		}
	}
	if compound.IsCompound {
		if _, err = asn1.Unmarshal(compound.Bytes, &content); err != nil {
			return nil, err
		}
	} else {
		content = compound.Bytes
	}

	return &PKCS7{
		Content:      content,
		Certificates: certs,
		CRLs:         crls,
		Signers:      sd.SignerInfos,
		raw:          sd,
	}, nil
}

func parseRevocationLists(rawCRLs []asn1.RawValue) ([]*x509.RevocationList, error) {
	if len(rawCRLs) == 0 {
		return nil, nil
	}

	crls := make([]*x509.RevocationList, 0, len(rawCRLs))
	for _, raw := range rawCRLs {
		der := raw.FullBytes
		if len(der) == 0 {
			der = raw.Bytes
		}
		if len(der) == 0 {
			continue
		}
		rl, err := x509.ParseRevocationList(der)
		if err != nil {
			return nil, err
		}
		crls = append(crls, rl)
	}
	return crls, nil
}

func (raw rawCertificates) Parse() ([]*x509.Certificate, error) {
	if len(raw.Raw) == 0 {
		return nil, nil
	}

	var val asn1.RawValue
	if _, err := asn1.Unmarshal(raw.Raw, &val); err != nil {
		return nil, err
	}

	return x509.ParseCertificates(val.Bytes)
}

// Verify checks signature integrity.
// WARNING: does not verify trust chains or signing time.
func (p7 *PKCS7) Verify() error {
	if len(p7.Signers) == 0 {
		return errors.New("pkcs7: Message has no signers")
	}
	for _, signer := range p7.Signers {
		if err := verifySignature(p7, signer); err != nil {
			return err
		}
	}
	return nil
}

func verifySignature(p7 *PKCS7, signer signerInfo) error {
	signedData := p7.Content
	hash, err := getHashForOID(signer.DigestAlgorithm.Algorithm)
	if err != nil {
		return err
	}
	if len(signer.AuthenticatedAttributes) > 0 {
		var digest []byte
		if err := unmarshalAttribute(signer.AuthenticatedAttributes, oidAttributeMessageDigest, &digest); err != nil {
			return err
		}
		h := hash.New()
		h.Write(p7.Content)
		computed := h.Sum(nil)
		if !hmac.Equal(digest, computed) {
			return &MessageDigestMismatchError{ExpectedDigest: digest, ActualDigest: computed}
		}
		var err error
		signedData, err = marshalAttributes(signer.AuthenticatedAttributes)
		if err != nil {
			return err
		}
	}

	cert := getCertFromCertsByIssuerAndSerial(p7.Certificates, signer.IssuerAndSerialNumber)
	if cert == nil {
		return errors.New("pkcs7: No certificate for signer")
	}

	algo := getSignatureAlgorithmFromAI(signer.DigestEncryptionAlgorithm)
	if algo == x509.UnknownSignatureAlgorithm {
		if signer.DigestEncryptionAlgorithm.Algorithm.Equal(oidEncryptionAlgorithmRSA) {
			algo = x509.SHA256WithRSA
		}
	}
	if algo == x509.UnknownSignatureAlgorithm {
		return errors.New("pkcs7: unsupported signature algorithm")
	}
	return cert.CheckSignature(algo, signedData, signer.EncryptedDigest)
}

func marshalAttributes(attrs []attribute) ([]byte, error) {
	encodedAttributes, err := asn1.Marshal(struct {
		A []attribute `asn1:"set"`
	}{A: attrs})
	if err != nil {
		return nil, err
	}

	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(encodedAttributes, &raw); err != nil {
		return nil, err
	}
	return raw.Bytes, nil
}

func getCertFromCertsByIssuerAndSerial(certs []*x509.Certificate, ias issuerAndSerial) *x509.Certificate {
	for _, cert := range certs {
		if isCertMatchForIssuerAndSerial(cert, ias) {
			return cert
		}
	}
	return nil
}

func isCertMatchForIssuerAndSerial(cert *x509.Certificate, ias issuerAndSerial) bool {
	if cert == nil || ias.SerialNumber == nil {
		return false
	}
	if cert.SerialNumber == nil || cert.SerialNumber.Cmp(ias.SerialNumber) != 0 {
		return false
	}

	issuerDER := ias.IssuerName.FullBytes
	if len(issuerDER) == 0 {
		issuerDER = ias.IssuerName.Bytes
	}
	return bytes.Equal(cert.RawIssuer, issuerDER)
}

func getHashForOID(oid asn1.ObjectIdentifier) (crypto.Hash, error) {
	if oid.Equal(oidSHA256) {
		return crypto.SHA256, nil
	}
	return crypto.Hash(0), errors.New("pkcs7: unsupported digest algorithm")
}

func (p7 *PKCS7) GetOnlySigner() *x509.Certificate {
	if len(p7.Signers) != 1 {
		return nil
	}
	signer := p7.Signers[0]
	return getCertFromCertsByIssuerAndSerial(p7.Certificates, signer.IssuerAndSerialNumber)
}

func unmarshalAttribute(attrs []attribute, attributeType asn1.ObjectIdentifier, out interface{}) error {
	for _, attr := range attrs {
		if attr.Type.Equal(attributeType) {
			_, err := asn1.Unmarshal(attr.Value.Bytes, out)
			return err
		}
	}
	return errors.New("pkcs7: attribute type not in attributes")
}

func (p7 *PKCS7) UnmarshalSignedAttribute(attributeType asn1.ObjectIdentifier, out interface{}) error {
	sd, ok := p7.raw.(signedData)
	if !ok {
		return errors.New("pkcs7: payload is not signedData content")
	}
	if len(sd.SignerInfos) < 1 {
		return errors.New("pkcs7: payload has no signers")
	}
	attributes := sd.SignerInfos[0].AuthenticatedAttributes
	return unmarshalAttribute(attributes, attributeType, out)
}

type SignedData struct {
	sd            signedData
	certs         []*x509.Certificate
	messageDigest []byte
}

type Attribute struct {
	Type  asn1.ObjectIdentifier
	Value interface{}
}

type SignerInfoConfig struct {
	ExtraSignedAttributes []Attribute
}

func NewSignedData(data []byte) (*SignedData, error) {
	content, err := asn1.Marshal(data)
	if err != nil {
		return nil, err
	}
	ci := contentInfo{
		ContentType: oidData,
		Content:     asn1.RawValue{Class: 2, Tag: 0, Bytes: content, IsCompound: true},
	}
	digAlg := pkix.AlgorithmIdentifier{Algorithm: oidSHA256}
	h := crypto.SHA256.New()
	h.Write(data)
	md := h.Sum(nil)
	sd := signedData{
		ContentInfo:                ci,
		Version:                    1,
		DigestAlgorithmIdentifiers: []pkix.AlgorithmIdentifier{digAlg},
	}
	return &SignedData{sd: sd, messageDigest: md}, nil
}

type attributes struct {
	types  []asn1.ObjectIdentifier
	values []interface{}
}

func (attrs *attributes) Add(attrType asn1.ObjectIdentifier, value interface{}) {
	attrs.types = append(attrs.types, attrType)
	attrs.values = append(attrs.values, value)
}

type sortableAttribute struct {
	SortKey   []byte
	Attribute attribute
}

type attributeSet []sortableAttribute

func (sa attributeSet) Len() int           { return len(sa) }
func (sa attributeSet) Less(i, j int) bool { return bytes.Compare(sa[i].SortKey, sa[j].SortKey) < 0 }
func (sa attributeSet) Swap(i, j int)      { sa[i], sa[j] = sa[j], sa[i] }

func (sa attributeSet) Attributes() []attribute {
	attrs := make([]attribute, len(sa))
	for i, attr := range sa {
		attrs[i] = attr.Attribute
	}
	return attrs
}

func (attrs *attributes) ForMarshaling() ([]attribute, error) {
	sortables := make(attributeSet, len(attrs.types))
	for i := range sortables {
		attrType := attrs.types[i]
		attrValue := attrs.values[i]
		asn1Value, err := asn1.Marshal(attrValue)
		if err != nil {
			return nil, err
		}
		attr := attribute{
			Type:  attrType,
			Value: asn1.RawValue{Tag: 17, IsCompound: true, Bytes: asn1Value},
		}
		encoded, err := asn1.Marshal(attr)
		if err != nil {
			return nil, err
		}
		sortables[i] = sortableAttribute{SortKey: encoded, Attribute: attr}
	}
	sort.Sort(sortables)
	return sortables.Attributes(), nil
}

func (sd *SignedData) AddSigner(cert *x509.Certificate, pkey crypto.PrivateKey, config SignerInfoConfig) error {
	attrs := &attributes{}
	attrs.Add(oidAttributeContentType, sd.sd.ContentInfo.ContentType)
	attrs.Add(oidAttributeMessageDigest, sd.messageDigest)
	attrs.Add(oidAttributeSigningTime, time.Now())
	for _, attr := range config.ExtraSignedAttributes {
		attrs.Add(attr.Type, attr.Value)
	}
	finalAttrs, err := attrs.ForMarshaling()
	if err != nil {
		return err
	}
	signature, err := signAttributes(finalAttrs, pkey)
	if err != nil {
		return err
	}

	ias, err := cert2issuerAndSerial(cert)
	if err != nil {
		return err
	}

	digestEncryptionAlgorithm, err := signatureAlgorithmOIDForPrivateKey(pkey)
	if err != nil {
		return err
	}

	signer := signerInfo{
		AuthenticatedAttributes:   finalAttrs,
		DigestAlgorithm:           pkix.AlgorithmIdentifier{Algorithm: oidSHA256},
		DigestEncryptionAlgorithm: pkix.AlgorithmIdentifier{Algorithm: digestEncryptionAlgorithm},
		IssuerAndSerialNumber:     ias,
		EncryptedDigest:           signature,
		Version:                   1,
	}
	sd.certs = append(sd.certs, cert)
	sd.sd.SignerInfos = append(sd.sd.SignerInfos, signer)
	return nil
}

func (sd *SignedData) AddCertificate(cert *x509.Certificate) { sd.certs = append(sd.certs, cert) }

func (sd *SignedData) Detach() { sd.sd.ContentInfo = contentInfo{ContentType: oidData} }

func (sd *SignedData) Finish() ([]byte, error) {
	sd.sd.Certificates = marshalCertificates(sd.certs)
	inner, err := asn1.Marshal(sd.sd)
	if err != nil {
		return nil, err
	}
	outer := contentInfo{
		ContentType: oidSignedData,
		Content:     asn1.RawValue{Class: 2, Tag: 0, Bytes: inner, IsCompound: true},
	}
	return asn1.Marshal(outer)
}

func cert2issuerAndSerial(cert *x509.Certificate) (issuerAndSerial, error) {
	return issuerAndSerial{
		IssuerName:   asn1.RawValue{FullBytes: cert.RawIssuer},
		SerialNumber: cert.SerialNumber,
	}, nil
}

func signAttributes(attrs []attribute, pkey crypto.PrivateKey) ([]byte, error) {
	attrBytes, err := marshalAttributes(attrs)
	if err != nil {
		return nil, err
	}
	h := crypto.SHA256.New()
	h.Write(attrBytes)
	hashed := h.Sum(nil)

	switch priv := pkey.(type) {
	case *rsa.PrivateKey:
		return rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed)
	case *ecdsa.PrivateKey:
		return ecdsa.SignASN1(rand.Reader, priv, hashed)
	default:
		return nil, errors.New("pkcs7: unsupported private key type")
	}
}

func signatureAlgorithmOIDForPrivateKey(pkey crypto.PrivateKey) (asn1.ObjectIdentifier, error) {
	switch pkey.(type) {
	case *rsa.PrivateKey:
		return oidSignatureSHA256WithRSA, nil
	case *ecdsa.PrivateKey:
		return oidSignatureECDSAWithSHA256, nil
	default:
		return nil, errors.New("pkcs7: unsupported private key type")
	}
}

func marshalCertificates(certs []*x509.Certificate) rawCertificates {
	var buf bytes.Buffer
	for _, cert := range certs {
		buf.Write(cert.Raw)
	}
	rawCerts, _ := marshalCertificateBytes(buf.Bytes())
	return rawCerts
}

func marshalCertificateBytes(certs []byte) (rawCertificates, error) {
	val := asn1.RawValue{Bytes: certs, Class: 2, Tag: 0, IsCompound: true}
	b, err := asn1.Marshal(val)
	if err != nil {
		return rawCertificates{}, err
	}
	return rawCertificates{Raw: b}, nil
}

func DegenerateCertificate(cert []byte) ([]byte, error) {
	rawCert, err := marshalCertificateBytes(cert)
	if err != nil {
		return nil, err
	}
	emptyContent := contentInfo{ContentType: oidData}
	sd := signedData{
		Version:      1,
		ContentInfo:  emptyContent,
		Certificates: rawCert,
	}
	content, err := asn1.Marshal(sd)
	if err != nil {
		return nil, err
	}
	signedContent := contentInfo{
		ContentType: oidSignedData,
		Content:     asn1.RawValue{Class: 2, Tag: 0, Bytes: content, IsCompound: true},
	}
	return asn1.Marshal(signedContent)
}

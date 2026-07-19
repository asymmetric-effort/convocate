package saml

import (
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// AuthnRequest represents a SAML AuthnRequest.
type AuthnRequest struct {
	XMLName                     xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol AuthnRequest"`
	ID                          string   `xml:"ID,attr"`
	Version                     string   `xml:"Version,attr"`
	IssueInstant                string   `xml:"IssueInstant,attr"`
	Destination                 string   `xml:"Destination,attr"`
	AssertionConsumerServiceURL string   `xml:"AssertionConsumerServiceURL,attr"`
	ProtocolBinding             string   `xml:"ProtocolBinding,attr"`
	Issuer                      AuthnRequestIssuer
}

// AuthnRequestIssuer is the issuer of the AuthnRequest.
type AuthnRequestIssuer struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Value   string   `xml:",chardata"`
}

// ParseAuthnRequestRedirect parses an AuthnRequest from a redirect binding (query parameter).
func ParseAuthnRequestRedirect(samlRequest string) (*AuthnRequest, error) {
	// URL decode
	decoded, err := url.QueryUnescape(samlRequest)
	if err != nil {
		decoded = samlRequest
	}

	// Base64 decode
	compressed, err := base64.StdEncoding.DecodeString(decoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	// DEFLATE decompress
	reader := flate.NewReader(strings.NewReader(string(compressed)))
	defer reader.Close()

	xmlData, err := io.ReadAll(reader)
	if err != nil {
		// Try without decompression (some SPs send uncompressed)
		xmlData = compressed
	}

	return parseAuthnRequestXML(xmlData)
}

// ParseAuthnRequestPost parses an AuthnRequest from a POST binding (form value).
func ParseAuthnRequestPost(samlRequest string) (*AuthnRequest, error) {
	// Base64 decode (POST binding is not deflated)
	xmlData, err := base64.StdEncoding.DecodeString(samlRequest)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	return parseAuthnRequestXML(xmlData)
}

func parseAuthnRequestXML(data []byte) (*AuthnRequest, error) {
	var req AuthnRequest
	if err := xml.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal AuthnRequest: %w", err)
	}

	if req.ID == "" {
		return nil, fmt.Errorf("AuthnRequest missing ID")
	}
	if req.Version != "2.0" {
		return nil, fmt.Errorf("unsupported SAML version: %s", req.Version)
	}

	return &req, nil
}

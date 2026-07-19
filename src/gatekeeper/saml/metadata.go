package saml

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
)

// EntityDescriptor represents SAML IdP metadata.
type EntityDescriptor struct {
	XMLName          xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	EntityID         string   `xml:"entityID,attr"`
	IDPSSODescriptor IDPSSODescriptor
}

// IDPSSODescriptor describes the IdP capabilities.
type IDPSSODescriptor struct {
	XMLName                    xml.Name              `xml:"urn:oasis:names:tc:SAML:2.0:metadata IDPSSODescriptor"`
	WantAuthnRequestsSigned    bool                  `xml:"WantAuthnRequestsSigned,attr"`
	ProtocolSupportEnumeration string                `xml:"protocolSupportEnumeration,attr"`
	KeyDescriptors             []KeyDescriptor       `xml:"urn:oasis:names:tc:SAML:2.0:metadata KeyDescriptor"`
	SingleSignOnServices       []SingleSignOnService `xml:"urn:oasis:names:tc:SAML:2.0:metadata SingleSignOnService"`
	SingleLogoutServices       []SingleLogoutService `xml:"urn:oasis:names:tc:SAML:2.0:metadata SingleLogoutService"`
	NameIDFormats              []NameIDFormat        `xml:"urn:oasis:names:tc:SAML:2.0:metadata NameIDFormat"`
}

// KeyDescriptor holds signing key info.
type KeyDescriptor struct {
	Use     string  `xml:"use,attr"`
	KeyInfo KeyInfo `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
}

// KeyInfo holds the X509 data.
type KeyInfo struct {
	X509Data X509Data `xml:"http://www.w3.org/2000/09/xmldsig# X509Data"`
}

// X509Data holds the certificate.
type X509Data struct {
	X509Certificate X509Certificate `xml:"http://www.w3.org/2000/09/xmldsig# X509Certificate"`
}

// X509Certificate holds the base64-encoded certificate.
type X509Certificate struct {
	Value string `xml:",chardata"`
}

// SingleSignOnService describes an SSO endpoint.
type SingleSignOnService struct {
	Binding  string `xml:"Binding,attr"`
	Location string `xml:"Location,attr"`
}

// SingleLogoutService describes an SLO endpoint.
type SingleLogoutService struct {
	Binding  string `xml:"Binding,attr"`
	Location string `xml:"Location,attr"`
}

// NameIDFormat describes supported name ID formats.
type NameIDFormat struct {
	Value string `xml:",chardata"`
}

// GenerateMetadata creates the IdP metadata XML.
func GenerateMetadata(entityID, ssoURL string, certDER []byte) ([]byte, error) {
	certB64 := base64.StdEncoding.EncodeToString(certDER)

	sloURL := ssoURL
	// Derive SLO URL from SSO URL by replacing /sso with /slo
	if len(ssoURL) > 4 && ssoURL[len(ssoURL)-4:] == "/sso" {
		sloURL = ssoURL[:len(ssoURL)-4] + "/slo"
	}

	descriptor := EntityDescriptor{
		EntityID: entityID,
		IDPSSODescriptor: IDPSSODescriptor{
			WantAuthnRequestsSigned:    false,
			ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
			KeyDescriptors: []KeyDescriptor{
				{
					Use: "signing",
					KeyInfo: KeyInfo{
						X509Data: X509Data{
							X509Certificate: X509Certificate{
								Value: certB64,
							},
						},
					},
				},
			},
			SingleSignOnServices: []SingleSignOnService{
				{
					Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
					Location: ssoURL,
				},
				{
					Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
					Location: ssoURL,
				},
			},
			SingleLogoutServices: []SingleLogoutService{
				{
					Binding:  "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
					Location: sloURL,
				},
			},
			NameIDFormats: []NameIDFormat{
				{Value: "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"},
				{Value: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"},
			},
		},
	}

	output, err := xml.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	result := append([]byte(xml.Header), output...)
	return result, nil
}

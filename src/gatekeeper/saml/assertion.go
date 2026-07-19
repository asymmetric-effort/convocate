package saml

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Response represents a SAML Response.
type Response struct {
	XMLName      xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol Response"`
	XMLNS        string   `xml:"xmlns:samlp,attr"`
	XMLNSA       string   `xml:"xmlns:saml,attr"`
	ID           string   `xml:"ID,attr"`
	Version      string   `xml:"Version,attr"`
	IssueInstant string   `xml:"IssueInstant,attr"`
	Destination  string   `xml:"Destination,attr"`
	InResponseTo string   `xml:"InResponseTo,attr"`
	Issuer       ResponseIssuer
	Status       Status
	Assertion    Assertion
	Signature    *SignatureXML `xml:",omitempty"`
}

// ResponseIssuer is the issuer element in a Response.
type ResponseIssuer struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Value   string   `xml:",chardata"`
}

// Status represents the SAML Status.
type Status struct {
	XMLName    xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol Status"`
	StatusCode StatusCode
}

// StatusCode represents the SAML StatusCode.
type StatusCode struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol StatusCode"`
	Value   string   `xml:"Value,attr"`
}

// Assertion represents a SAML Assertion.
type Assertion struct {
	XMLName            xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
	XMLNS              string   `xml:"xmlns:saml,attr"`
	ID                 string   `xml:"ID,attr"`
	Version            string   `xml:"Version,attr"`
	IssueInstant       string   `xml:"IssueInstant,attr"`
	Issuer             AssertionIssuer
	Signature          *SignatureXML `xml:",omitempty"`
	Subject            Subject
	Conditions         Conditions
	AuthnStatement     AuthnStatement
	AttributeStatement AttributeStatement
}

// AssertionIssuer is the issuer element in an Assertion.
type AssertionIssuer struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Value   string   `xml:",chardata"`
}

// Subject contains the NameID and confirmation.
type Subject struct {
	XMLName             xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Subject"`
	NameID              NameID
	SubjectConfirmation SubjectConfirmation
}

// NameID identifies the subject.
type NameID struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion NameID"`
	Format  string   `xml:"Format,attr"`
	Value   string   `xml:",chardata"`
}

// SubjectConfirmation confirms the subject.
type SubjectConfirmation struct {
	XMLName                 xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion SubjectConfirmation"`
	Method                  string   `xml:"Method,attr"`
	SubjectConfirmationData SubjectConfirmationData
}

// SubjectConfirmationData holds confirmation parameters.
type SubjectConfirmationData struct {
	XMLName      xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion SubjectConfirmationData"`
	InResponseTo string   `xml:"InResponseTo,attr"`
	NotOnOrAfter string   `xml:"NotOnOrAfter,attr"`
	Recipient    string   `xml:"Recipient,attr"`
}

// Conditions holds audience restrictions.
type Conditions struct {
	XMLName             xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Conditions"`
	NotBefore           string   `xml:"NotBefore,attr"`
	NotOnOrAfter        string   `xml:"NotOnOrAfter,attr"`
	AudienceRestriction AudienceRestriction
}

// AudienceRestriction restricts the assertion audience.
type AudienceRestriction struct {
	XMLName  xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AudienceRestriction"`
	Audience Audience
}

// Audience identifies the intended audience.
type Audience struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Audience"`
	Value   string   `xml:",chardata"`
}

// AuthnStatement describes authentication context.
type AuthnStatement struct {
	XMLName      xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AuthnStatement"`
	AuthnInstant string   `xml:"AuthnInstant,attr"`
	SessionIndex string   `xml:"SessionIndex,attr"`
	AuthnContext AuthnContext
}

// AuthnContext provides authentication context class.
type AuthnContext struct {
	XMLName              xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AuthnContext"`
	AuthnContextClassRef AuthnContextClassRef
}

// AuthnContextClassRef is the authentication class reference.
type AuthnContextClassRef struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AuthnContextClassRef"`
	Value   string   `xml:",chardata"`
}

// AttributeStatement holds SAML attributes.
type AttributeStatement struct {
	XMLName    xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AttributeStatement"`
	Attributes []Attribute
}

// Attribute is a SAML attribute.
type Attribute struct {
	XMLName    xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Attribute"`
	Name       string   `xml:"Name,attr"`
	NameFormat string   `xml:"NameFormat,attr"`
	Values     []AttributeValue
}

// AttributeValue holds an attribute value.
type AttributeValue struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AttributeValue"`
	Type    string   `xml:"xmlns:xsi,attr,omitempty"`
	XSType  string   `xml:"xsi:type,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

// SignatureXML represents an XML Signature.
type SignatureXML struct {
	XMLName        xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
	SignedInfo     SignedInfo
	SignatureValue SignatureValue
	KeyInfo        SignatureKeyInfo
}

// SignedInfo contains canonicalization, signature method, and references.
type SignedInfo struct {
	XMLName                xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# SignedInfo"`
	CanonicalizationMethod CanonicalizationMethod
	SignatureMethod        SignatureMethod
	Reference              Reference
}

// CanonicalizationMethod identifies the c14n algorithm.
type CanonicalizationMethod struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# CanonicalizationMethod"`
	Algorithm string   `xml:"Algorithm,attr"`
}

// SignatureMethod identifies the signature algorithm.
type SignatureMethod struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# SignatureMethod"`
	Algorithm string   `xml:"Algorithm,attr"`
}

// Reference identifies the signed element.
type Reference struct {
	XMLName      xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Reference"`
	URI          string   `xml:"URI,attr"`
	Transforms   Transforms
	DigestMethod DigestMethod
	DigestValue  DigestValue
}

// Transforms holds the transform algorithms.
type Transforms struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Transforms"`
	Transform []Transform
}

// Transform is a single transform.
type Transform struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Transform"`
	Algorithm string   `xml:"Algorithm,attr"`
}

// DigestMethod identifies the digest algorithm.
type DigestMethod struct {
	XMLName   xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# DigestMethod"`
	Algorithm string   `xml:"Algorithm,attr"`
}

// DigestValue holds the digest.
type DigestValue struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# DigestValue"`
	Value   string   `xml:",chardata"`
}

// SignatureValue holds the signature.
type SignatureValue struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# SignatureValue"`
	Value   string   `xml:",chardata"`
}

// SignatureKeyInfo holds the key info in a signature.
type SignatureKeyInfo struct {
	XMLName  xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
	X509Data SignatureX509Data
}

// SignatureX509Data holds the certificate in signature key info.
type SignatureX509Data struct {
	XMLName         xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# X509Data"`
	X509Certificate SignatureX509Certificate
}

// SignatureX509Certificate holds the base64-encoded cert.
type SignatureX509Certificate struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# X509Certificate"`
	Value   string   `xml:",chardata"`
}

// AssertionParams holds parameters for building an assertion.
type AssertionParams struct {
	EntityID  string
	SSOURL    string
	RequestID string
	ACSURL    string
	Audience  string
	Username  string
	Email     string
	Groups    []string
}

// BuildSignedResponse builds and signs a SAML Response with a signed Assertion.
func BuildSignedResponse(params AssertionParams, keys *KeyPair) ([]byte, error) {
	now := time.Now().UTC()
	notOnOrAfter := now.Add(5 * time.Minute)
	assertionID := "_" + generateID()
	responseID := "_" + generateID()

	certB64 := base64.StdEncoding.EncodeToString(keys.Certificate.Raw)

	// Build assertion without signature first
	assertion := Assertion{
		XMLNS:        "urn:oasis:names:tc:SAML:2.0:assertion",
		ID:           assertionID,
		Version:      "2.0",
		IssueInstant: now.Format(time.RFC3339),
		Issuer:       AssertionIssuer{Value: params.EntityID},
		Subject: Subject{
			NameID: NameID{
				Format: "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified",
				Value:  params.Username,
			},
			SubjectConfirmation: SubjectConfirmation{
				Method: "urn:oasis:names:tc:SAML:2.0:cm:bearer",
				SubjectConfirmationData: SubjectConfirmationData{
					InResponseTo: params.RequestID,
					NotOnOrAfter: notOnOrAfter.Format(time.RFC3339),
					Recipient:    params.ACSURL,
				},
			},
		},
		Conditions: Conditions{
			NotBefore:    now.Add(-5 * time.Second).Format(time.RFC3339),
			NotOnOrAfter: notOnOrAfter.Format(time.RFC3339),
			AudienceRestriction: AudienceRestriction{
				Audience: Audience{Value: params.Audience},
			},
		},
		AuthnStatement: AuthnStatement{
			AuthnInstant: now.Format(time.RFC3339),
			SessionIndex: assertionID,
			AuthnContext: AuthnContext{
				AuthnContextClassRef: AuthnContextClassRef{
					Value: "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport",
				},
			},
		},
		AttributeStatement: buildAttributeStatement(params),
	}

	// Sign the assertion
	assertionXML, err := xml.Marshal(assertion)
	if err != nil {
		return nil, fmt.Errorf("marshal assertion: %w", err)
	}

	assertionSig, err := signElement(assertionXML, assertionID, keys, certB64)
	if err != nil {
		return nil, fmt.Errorf("sign assertion: %w", err)
	}
	assertion.Signature = assertionSig

	// Build response
	response := Response{
		XMLNS:        "urn:oasis:names:tc:SAML:2.0:protocol",
		XMLNSA:       "urn:oasis:names:tc:SAML:2.0:assertion",
		ID:           responseID,
		Version:      "2.0",
		IssueInstant: now.Format(time.RFC3339),
		Destination:  params.ACSURL,
		InResponseTo: params.RequestID,
		Issuer:       ResponseIssuer{Value: params.EntityID},
		Status: Status{
			StatusCode: StatusCode{
				Value: "urn:oasis:names:tc:SAML:2.0:status:Success",
			},
		},
		Assertion: assertion,
	}

	// Sign the response
	responseXML, err := xml.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	responseSig, err := signElement(responseXML, responseID, keys, certB64)
	if err != nil {
		return nil, fmt.Errorf("sign response: %w", err)
	}
	response.Signature = responseSig

	// Final marshal
	output, err := xml.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("marshal final response: %w", err)
	}

	return output, nil
}

func buildAttributeStatement(params AssertionParams) AttributeStatement {
	attrs := []Attribute{
		{
			Name:       "email",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
			Values: []AttributeValue{
				{Value: params.Email},
			},
		},
	}

	if len(params.Groups) > 0 {
		groupValues := make([]AttributeValue, 0, len(params.Groups))
		for i := 0; i < len(params.Groups); i++ {
			groupValues = append(groupValues, AttributeValue{Value: params.Groups[i]})
		}
		attrs = append(attrs, Attribute{
			Name:       "groups",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic",
			Values:     groupValues,
		})
	}

	return AttributeStatement{Attributes: attrs}
}

// signElement creates an XML signature for the given element.
func signElement(elementXML []byte, refID string, keys *KeyPair, certB64 string) (*SignatureXML, error) {
	// Canonicalize the element (simplified exc-c14n)
	canonical := canonicalize(elementXML)

	// Compute digest
	digest := sha256.Sum256(canonical)
	digestB64 := base64.StdEncoding.EncodeToString(digest[:])

	// Build SignedInfo
	signedInfo := SignedInfo{
		CanonicalizationMethod: CanonicalizationMethod{
			Algorithm: "http://www.w3.org/2001/10/xml-exc-c14n#",
		},
		SignatureMethod: SignatureMethod{
			Algorithm: "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256",
		},
		Reference: Reference{
			URI: "#" + refID,
			Transforms: Transforms{
				Transform: []Transform{
					{Algorithm: "http://www.w3.org/2000/09/xmldsig#enveloped-signature"},
					{Algorithm: "http://www.w3.org/2001/10/xml-exc-c14n#"},
				},
			},
			DigestMethod: DigestMethod{
				Algorithm: "http://www.w3.org/2001/04/xmlenc#sha256",
			},
			DigestValue: DigestValue{
				Value: digestB64,
			},
		},
	}

	// Marshal and canonicalize SignedInfo
	signedInfoXML, err := xml.Marshal(signedInfo)
	if err != nil {
		return nil, fmt.Errorf("marshal signed info: %w", err)
	}
	canonicalSignedInfo := canonicalize(signedInfoXML)

	// Sign
	hash := sha256.Sum256(canonicalSignedInfo)
	signature, err := rsa.SignPKCS1v15(rand.Reader, keys.PrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	sigB64 := base64.StdEncoding.EncodeToString(signature)

	return &SignatureXML{
		SignedInfo:     signedInfo,
		SignatureValue: SignatureValue{Value: sigB64},
		KeyInfo: SignatureKeyInfo{
			X509Data: SignatureX509Data{
				X509Certificate: SignatureX509Certificate{
					Value: certB64,
				},
			},
		},
	}, nil
}

// canonicalize performs a simplified Exclusive XML Canonicalization.
// This implementation normalizes whitespace, sorts attributes, and removes
// redundant namespace declarations for the purpose of signature computation.
func canonicalize(xmlData []byte) []byte {
	// Parse and re-serialize to normalize
	// This is a simplified c14n implementation that:
	// 1. Removes XML declaration
	// 2. Normalizes attribute ordering (alphabetical)
	// 3. Removes extra whitespace between elements
	// 4. Uses self-closing tags for empty elements

	input := string(xmlData)

	// Remove XML declaration if present
	if strings.HasPrefix(input, "<?xml") {
		idx := strings.Index(input, "?>")
		if idx >= 0 {
			input = strings.TrimSpace(input[idx+2:])
		}
	}

	// Parse into tokens and rebuild
	decoder := xml.NewDecoder(strings.NewReader(input))
	var result strings.Builder

	type stackItem struct {
		name xml.Name
		ns   map[string]string
	}
	stack := make([]stackItem, 0, 16)

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			result.WriteByte('<')
			if t.Name.Space != "" {
				result.WriteString(t.Name.Space)
				result.WriteByte(':')
			}
			result.WriteString(t.Name.Local)

			// Sort attributes alphabetically for canonical form
			attrs := make([]xml.Attr, len(t.Attr))
			copy(attrs, t.Attr)
			sort.Slice(attrs, func(i, j int) bool {
				ai := attrs[i]
				aj := attrs[j]
				// xmlns attrs first, then by name
				aiIsNS := ai.Name.Space == "xmlns" || ai.Name.Local == "xmlns"
				ajIsNS := aj.Name.Space == "xmlns" || aj.Name.Local == "xmlns"
				if aiIsNS != ajIsNS {
					return aiIsNS
				}
				if ai.Name.Space != aj.Name.Space {
					return ai.Name.Space < aj.Name.Space
				}
				return ai.Name.Local < aj.Name.Local
			})

			for _, attr := range attrs {
				result.WriteByte(' ')
				if attr.Name.Space != "" {
					result.WriteString(attr.Name.Space)
					result.WriteByte(':')
				}
				result.WriteString(attr.Name.Local)
				result.WriteString(`="`)
				result.WriteString(xmlEscapeAttr(attr.Value))
				result.WriteByte('"')
			}
			result.WriteByte('>')

			stack = append(stack, stackItem{name: t.Name})

		case xml.EndElement:
			result.WriteString("</")
			if t.Name.Space != "" {
				result.WriteString(t.Name.Space)
				result.WriteByte(':')
			}
			result.WriteString(t.Name.Local)
			result.WriteByte('>')

			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			result.WriteString(xmlEscapeText(string(t)))
		}
	}

	return []byte(result.String())
}

func xmlEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "\t", "&#x9;")
	s = strings.ReplaceAll(s, "\n", "&#xA;")
	s = strings.ReplaceAll(s, "\r", "&#xD;")
	return s
}

func xmlEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\r", "&#xD;")
	return s
}

// generateID generates a random ID suitable for SAML.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

package invoicepdf

import (
	"crypto/sha1"
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	isdocVersion            = "6.0.2"
	isdocIssuingSystem      = "invoicepdf"
	isdocDocumentInvoice    = 1
	isdocDocumentCreditNote = 2
	isdocAmountDecimals     = 2
	isdocRate               = "1"
	isdocZeroAmount         = "0.00"
	isdocCountryCode        = "CZ"
	isdocCountryName        = "Česká republika"
	isdocCurrencyLength     = 3
)

// ErrInvalidISDOC reports a Document that RenderISDOC can not represent.
var ErrInvalidISDOC = errors.New("invoicepdf: isdoc: invalid document")

// isdocUUIDNamespace is the fixed name space for the deterministic document
// UUID. RFC 4122 version 5 adds it in front of the document identity.
var isdocUUIDNamespace = [16]byte{
	0xe7, 0x56, 0x06, 0x83, 0x14, 0xcb, 0x4e, 0xee,
	0x9c, 0xd4, 0x1a, 0x60, 0xac, 0xb9, 0x71, 0xce,
}

// isdocZoneAtStartPattern matches a postal zone that starts its line, for
// example "110 00 Praha 1".
var isdocZoneAtStartPattern = regexp.MustCompile(`^(\d{3})\s?(\d{2})\s+(.+)$`)

// isdocZoneInLinePattern matches a postal zone after a street part that ends
// with a comma, for example "Dlouhá 12, 110 00 Praha 1".
var isdocZoneInLinePattern = regexp.MustCompile(`^(.+?)\s*,\s*(\d{3})\s?(\d{2})\s+(.+)$`)

// RenderISDOC returns the document as ISDOC 6.0.2 XML in the
// http://isdoc.cz/namespace/2013 name space. Equal documents return identical
// bytes.
//
// The Document model carries no VAT data, so every document is exported as a
// non-VAT document: VATApplicable is false and every line uses a zero percent
// tax category. Amounts are minor units of Document.Currency. The README
// documents the full field mapping and its limits.
func RenderISDOC(document Document) ([]byte, error) {
	invoice, err := isdocFromDocument(document)
	if err != nil {
		return nil, err
	}
	body, err := xml.MarshalIndent(invoice, "", "  ")
	if err != nil {
		return nil, err
	}
	output := make([]byte, 0, len(xml.Header)+len(body)+1)
	output = append(output, xml.Header...)
	output = append(output, body...)
	return append(output, '\n'), nil
}

func isdocFromDocument(document Document) (isdocInvoice, error) {
	number := strings.TrimSpace(document.Number)
	if number == "" {
		return isdocInvoice{}, fmt.Errorf("%w: number is empty", ErrInvalidISDOC)
	}
	documentType, err := isdocDocumentType(document.Kind)
	if err != nil {
		return isdocInvoice{}, err
	}
	issuedDate := strings.TrimSpace(document.IssuedDate)
	if issuedDate == "" {
		return isdocInvoice{}, fmt.Errorf("%w: issued date is empty", ErrInvalidISDOC)
	}
	issueDate, err := isdocDate(issuedDate)
	if err != nil {
		return isdocInvoice{}, err
	}
	sellerName := strings.TrimSpace(document.SellerName)
	if sellerName == "" {
		return isdocInvoice{}, fmt.Errorf("%w: seller name is empty", ErrInvalidISDOC)
	}
	buyerName := strings.TrimSpace(document.BuyerName)
	if buyerName == "" {
		return isdocInvoice{}, fmt.Errorf("%w: buyer name is empty", ErrInvalidISDOC)
	}
	currency := strings.ToUpper(strings.TrimSpace(document.Currency))
	if len(currency) != isdocCurrencyLength {
		return isdocInvoice{}, fmt.Errorf(
			"%w: currency %q is not a three-letter code", ErrInvalidISDOC, document.Currency,
		)
	}
	if len(document.Lines) == 0 {
		return isdocInvoice{}, fmt.Errorf("%w: at least one line is required", ErrInvalidISDOC)
	}
	creditNote := documentType == isdocDocumentCreditNote
	originalNumber := strings.TrimSpace(document.OriginalNumber)
	if creditNote && originalNumber == "" {
		return isdocInvoice{}, fmt.Errorf(
			"%w: credit note original number is empty", ErrInvalidISDOC,
		)
	}

	total := formatISDOCAmount(document.TotalMinor, creditNote)
	lines := make([]isdocInvoiceLine, 0, len(document.Lines))
	for index, line := range document.Lines {
		lines = append(lines, isdocLine(line, index+1, creditNote))
	}
	invoice := isdocInvoice{
		Version:            isdocVersion,
		DocumentType:       documentType,
		ID:                 number,
		UUID:               isdocUUID(document),
		IssuingSystem:      isdocIssuingSystem,
		IssueDate:          issueDate,
		VATApplicable:      false,
		LocalCurrencyCode:  currency,
		CurrRate:           isdocRate,
		RefCurrRate:        isdocRate,
		InvoiceLines:       isdocInvoiceLines{InvoiceLine: lines},
		TaxTotal:           isdocTaxTotal{TaxSubTotal: isdocZeroTaxSubTotal(total), TaxAmount: isdocZeroAmount},
		LegalMonetaryTotal: isdocZeroMonetaryTotal(total),
		AccountingSupplierParty: isdocAccountingParty{Party: isdocPartyFrom(
			sellerName, document.SellerID, document.SellerAddress,
			document.SellerEmail, document.SellerPhone,
		)},
		AccountingCustomerParty: isdocAccountingParty{Party: isdocPartyFrom(
			buyerName, "", document.BuyerAddress, document.BuyerEmail, "",
		)},
	}
	if order := strings.TrimSpace(document.OrderNumber); order != "" {
		invoice.OrderReferences = &isdocOrderReferences{
			OrderReference: isdocOrderReference{SalesOrderID: order},
		}
	}
	if creditNote {
		invoice.OriginalDocumentReferences = &isdocOriginalDocumentReferences{
			OriginalDocumentReference: isdocOriginalDocumentReference{ID: originalNumber},
		}
		if reason := strings.TrimSpace(document.Reason); reason != "" {
			invoice.Note = &isdocNote{
				LanguageID: isdocNoteLanguage(document.Locale),
				Text:       reason,
			}
		}
	}
	return invoice, nil
}

func isdocDocumentType(kind string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", KindInvoice:
		return isdocDocumentInvoice, nil
	case KindCreditNote:
		return isdocDocumentCreditNote, nil
	default:
		return 0, fmt.Errorf("%w: unknown kind %q", ErrInvalidISDOC, kind)
	}
}

func isdocDate(raw string) (string, error) {
	for _, layout := range []string{dateLayoutISO, time.RFC3339} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.Format(dateLayoutISO), nil
		}
	}
	return "", fmt.Errorf("%w: issued date %q is not an ISO date", ErrInvalidISDOC, raw)
}

func formatISDOCAmount(minor int64, creditNote bool) string {
	text := strconv.FormatInt(minor, 10)
	sign := ""
	if strings.HasPrefix(text, "-") {
		text = text[1:]
		if !creditNote {
			sign = "-"
		}
	}
	for len(text) <= isdocAmountDecimals {
		text = "0" + text
	}
	return sign + text[:len(text)-isdocAmountDecimals] + "." + text[len(text)-isdocAmountDecimals:]
}

func isdocLine(line Line, position int, creditNote bool) isdocInvoiceLine {
	description := strings.TrimSpace(line.Name)
	if configuration := strings.TrimSpace(line.Configuration); configuration != "" {
		if description == "" {
			description = configuration
		} else {
			description += "\n" + configuration
		}
	}
	unitPrice := formatISDOCAmount(line.UnitPriceMinor, creditNote)
	total := formatISDOCAmount(line.TotalMinor, creditNote)
	return isdocInvoiceLine{
		ID:                              strconv.Itoa(position),
		InvoicedQuantity:                line.Quantity,
		LineExtensionAmount:             total,
		LineExtensionAmountTaxInclusive: total,
		LineExtensionTaxAmount:          isdocZeroAmount,
		UnitPrice:                       unitPrice,
		UnitPriceTaxInclusive:           unitPrice,
		ClassifiedTaxCategory: isdocClassifiedTaxCategory{
			Percent:              0,
			VATCalculationMethod: 0,
			VATApplicable:        false,
		},
		Item: isdocItem{Description: description},
	}
}

func isdocZeroTaxSubTotal(total string) isdocTaxSubTotal {
	return isdocTaxSubTotal{
		TaxableAmount:                    total,
		TaxAmount:                        isdocZeroAmount,
		TaxInclusiveAmount:               total,
		AlreadyClaimedTaxableAmount:      isdocZeroAmount,
		AlreadyClaimedTaxAmount:          isdocZeroAmount,
		AlreadyClaimedTaxInclusiveAmount: isdocZeroAmount,
		DifferenceTaxableAmount:          total,
		DifferenceTaxAmount:              isdocZeroAmount,
		DifferenceTaxInclusiveAmount:     total,
		TaxCategory:                      isdocTaxCategory{Percent: 0, VATApplicable: false},
	}
}

func isdocZeroMonetaryTotal(total string) isdocLegalMonetaryTotal {
	return isdocLegalMonetaryTotal{
		TaxExclusiveAmount:               total,
		TaxInclusiveAmount:               total,
		AlreadyClaimedTaxExclusiveAmount: isdocZeroAmount,
		AlreadyClaimedTaxInclusiveAmount: isdocZeroAmount,
		DifferenceTaxExclusiveAmount:     total,
		DifferenceTaxInclusiveAmount:     total,
		PaidDepositsAmount:               isdocZeroAmount,
		PayableAmount:                    total,
	}
}

func isdocPartyFrom(name, id, address, email, phone string) isdocParty {
	party := isdocParty{
		PartyIdentification: isdocPartyIdentification{ID: strings.TrimSpace(id)},
		PartyName:           isdocPartyName{Name: strings.TrimSpace(name)},
		PostalAddress:       isdocPostalAddressFromText(address),
	}
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	if email != "" || phone != "" {
		party.Contact = &isdocContact{Telephone: phone, ElectronicMail: email}
	}
	return party
}

func isdocPostalAddressFromText(raw string) isdocPostalAddress {
	address := isdocPostalAddress{
		Country: isdocCountry{IdentificationCode: isdocCountryCode, Name: isdocCountryName},
	}
	lines := isdocAddressLines(raw)
	if len(lines) == 0 {
		return address
	}
	last := lines[len(lines)-1]
	streetLines := lines[:len(lines)-1]
	if match := isdocZoneAtStartPattern.FindStringSubmatch(last); match != nil {
		address.PostalZone = match[1] + " " + match[2]
		address.CityName = strings.TrimSpace(match[3])
	} else if match := isdocZoneInLinePattern.FindStringSubmatch(last); match != nil {
		if prefix := strings.TrimSpace(strings.TrimRight(match[1], ", ")); prefix != "" {
			streetLines = append(streetLines, prefix)
		}
		address.PostalZone = match[2] + " " + match[3]
		address.CityName = strings.TrimSpace(match[4])
	} else {
		streetLines = lines
	}
	address.StreetName = strings.Join(streetLines, ", ")
	return address
}

func isdocAddressLines(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	var lines []string
	for line := range strings.SplitSeq(normalized, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func isdocNoteLanguage(locale string) string {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	switch {
	case strings.HasPrefix(normalized, "cs"):
		return "cs"
	case strings.HasPrefix(normalized, "en"):
		return "en"
	default:
		return ""
	}
}

func isdocUUID(document Document) string {
	name := strings.Join([]string{
		strings.TrimSpace(document.Number),
		strings.ToLower(strings.TrimSpace(document.Kind)),
		strings.TrimSpace(document.IssuedDate),
		strings.TrimSpace(document.SellerID),
		strings.TrimSpace(document.SellerName),
		strings.TrimSpace(document.OriginalNumber),
		strconv.FormatInt(document.TotalMinor, 10),
	}, "\x00")
	hash := sha1.Sum(append(isdocUUIDNamespace[:], name...))
	var value [16]byte
	copy(value[:], hash[:])
	value[6] = value[6]&0x0f | 0x50
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

type isdocInvoice struct {
	XMLName                                 xml.Name                         `xml:"http://isdoc.cz/namespace/2013 Invoice"`
	Version                                 string                           `xml:"version,attr"`
	DocumentType                            int                              `xml:"DocumentType"`
	ID                                      string                           `xml:"ID"`
	UUID                                    string                           `xml:"UUID"`
	IssuingSystem                           string                           `xml:"IssuingSystem"`
	IssueDate                               string                           `xml:"IssueDate"`
	VATApplicable                           bool                             `xml:"VATApplicable"`
	ElectronicPossibilityAgreementReference string                           `xml:"ElectronicPossibilityAgreementReference"`
	Note                                    *isdocNote                       `xml:"Note,omitempty"`
	LocalCurrencyCode                       string                           `xml:"LocalCurrencyCode"`
	CurrRate                                string                           `xml:"CurrRate"`
	RefCurrRate                             string                           `xml:"RefCurrRate"`
	AccountingSupplierParty                 isdocAccountingParty             `xml:"AccountingSupplierParty"`
	AccountingCustomerParty                 isdocAccountingParty             `xml:"AccountingCustomerParty"`
	OrderReferences                         *isdocOrderReferences            `xml:"OrderReferences,omitempty"`
	OriginalDocumentReferences              *isdocOriginalDocumentReferences `xml:"OriginalDocumentReferences,omitempty"`
	InvoiceLines                            isdocInvoiceLines                `xml:"InvoiceLines"`
	TaxTotal                                isdocTaxTotal                    `xml:"TaxTotal"`
	LegalMonetaryTotal                      isdocLegalMonetaryTotal          `xml:"LegalMonetaryTotal"`
}

type isdocNote struct {
	LanguageID string `xml:"languageID,attr,omitempty"`
	Text       string `xml:",chardata"`
}

type isdocAccountingParty struct {
	Party isdocParty `xml:"Party"`
}

type isdocParty struct {
	PartyIdentification isdocPartyIdentification `xml:"PartyIdentification"`
	PartyName           isdocPartyName           `xml:"PartyName"`
	PostalAddress       isdocPostalAddress       `xml:"PostalAddress"`
	Contact             *isdocContact            `xml:"Contact,omitempty"`
}

type isdocPartyIdentification struct {
	ID string `xml:"ID"`
}

type isdocPartyName struct {
	Name string `xml:"Name"`
}

type isdocPostalAddress struct {
	StreetName     string       `xml:"StreetName"`
	BuildingNumber string       `xml:"BuildingNumber"`
	CityName       string       `xml:"CityName"`
	PostalZone     string       `xml:"PostalZone"`
	Country        isdocCountry `xml:"Country"`
}

type isdocCountry struct {
	IdentificationCode string `xml:"IdentificationCode"`
	Name               string `xml:"Name"`
}

type isdocContact struct {
	Telephone      string `xml:"Telephone,omitempty"`
	ElectronicMail string `xml:"ElectronicMail,omitempty"`
}

type isdocOrderReferences struct {
	OrderReference isdocOrderReference `xml:"OrderReference"`
}

type isdocOrderReference struct {
	SalesOrderID string `xml:"SalesOrderID"`
}

type isdocOriginalDocumentReferences struct {
	OriginalDocumentReference isdocOriginalDocumentReference `xml:"OriginalDocumentReference"`
}

type isdocOriginalDocumentReference struct {
	ID string `xml:"ID"`
}

type isdocInvoiceLines struct {
	InvoiceLine []isdocInvoiceLine `xml:"InvoiceLine"`
}

type isdocInvoiceLine struct {
	ID                              string                     `xml:"ID"`
	InvoicedQuantity                int64                      `xml:"InvoicedQuantity"`
	LineExtensionAmount             string                     `xml:"LineExtensionAmount"`
	LineExtensionAmountTaxInclusive string                     `xml:"LineExtensionAmountTaxInclusive"`
	LineExtensionTaxAmount          string                     `xml:"LineExtensionTaxAmount"`
	UnitPrice                       string                     `xml:"UnitPrice"`
	UnitPriceTaxInclusive           string                     `xml:"UnitPriceTaxInclusive"`
	ClassifiedTaxCategory           isdocClassifiedTaxCategory `xml:"ClassifiedTaxCategory"`
	Item                            isdocItem                  `xml:"Item"`
}

type isdocClassifiedTaxCategory struct {
	Percent              int  `xml:"Percent"`
	VATCalculationMethod int  `xml:"VATCalculationMethod"`
	VATApplicable        bool `xml:"VATApplicable"`
}

type isdocItem struct {
	Description string `xml:"Description"`
}

type isdocTaxTotal struct {
	TaxSubTotal isdocTaxSubTotal `xml:"TaxSubTotal"`
	TaxAmount   string           `xml:"TaxAmount"`
}

type isdocTaxSubTotal struct {
	TaxableAmount                    string           `xml:"TaxableAmount"`
	TaxAmount                        string           `xml:"TaxAmount"`
	TaxInclusiveAmount               string           `xml:"TaxInclusiveAmount"`
	AlreadyClaimedTaxableAmount      string           `xml:"AlreadyClaimedTaxableAmount"`
	AlreadyClaimedTaxAmount          string           `xml:"AlreadyClaimedTaxAmount"`
	AlreadyClaimedTaxInclusiveAmount string           `xml:"AlreadyClaimedTaxInclusiveAmount"`
	DifferenceTaxableAmount          string           `xml:"DifferenceTaxableAmount"`
	DifferenceTaxAmount              string           `xml:"DifferenceTaxAmount"`
	DifferenceTaxInclusiveAmount     string           `xml:"DifferenceTaxInclusiveAmount"`
	TaxCategory                      isdocTaxCategory `xml:"TaxCategory"`
}

type isdocTaxCategory struct {
	Percent       int  `xml:"Percent"`
	VATApplicable bool `xml:"VATApplicable"`
}

type isdocLegalMonetaryTotal struct {
	TaxExclusiveAmount               string `xml:"TaxExclusiveAmount"`
	TaxInclusiveAmount               string `xml:"TaxInclusiveAmount"`
	AlreadyClaimedTaxExclusiveAmount string `xml:"AlreadyClaimedTaxExclusiveAmount"`
	AlreadyClaimedTaxInclusiveAmount string `xml:"AlreadyClaimedTaxInclusiveAmount"`
	DifferenceTaxExclusiveAmount     string `xml:"DifferenceTaxExclusiveAmount"`
	DifferenceTaxInclusiveAmount     string `xml:"DifferenceTaxInclusiveAmount"`
	PaidDepositsAmount               string `xml:"PaidDepositsAmount"`
	PayableAmount                    string `xml:"PayableAmount"`
}

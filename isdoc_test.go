package invoicepdf_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/m1chlcz/invoicepdf"
)

// isdocInvoiceGolden is the full ISDOC 6.0.2 output of goldenDocument. The
// XML validates against the official isdoc-invoice-6.0.2.xsd from isdoc.cz.
const isdocInvoiceGolden = `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="http://isdoc.cz/namespace/2013" version="6.0.2">
  <DocumentType>1</DocumentType>
  <ID>2026-0042</ID>
  <UUID>92fedc9e-e458-5b81-ac5f-2a697f41d192</UUID>
  <IssuingSystem>invoicepdf</IssuingSystem>
  <IssueDate>2026-09-19</IssueDate>
  <VATApplicable>false</VATApplicable>
  <ElectronicPossibilityAgreementReference></ElectronicPossibilityAgreementReference>
  <LocalCurrencyCode>CZK</LocalCurrencyCode>
  <CurrRate>1</CurrRate>
  <RefCurrRate>1</RefCurrRate>
  <AccountingSupplierParty>
    <Party>
      <PartyIdentification>
        <ID>12345678</ID>
      </PartyIdentification>
      <PartyName>
        <Name>Sample Goods s.r.o.</Name>
      </PartyName>
      <PostalAddress>
        <StreetName>Dlouhá 12</StreetName>
        <BuildingNumber></BuildingNumber>
        <CityName>Praha 1</CityName>
        <PostalZone>110 00</PostalZone>
        <Country>
          <IdentificationCode>CZ</IdentificationCode>
          <Name>Česká republika</Name>
        </Country>
      </PostalAddress>
      <Contact>
        <Telephone>+420 123 456 789</Telephone>
        <ElectronicMail>billing@example.com</ElectronicMail>
      </Contact>
    </Party>
  </AccountingSupplierParty>
  <AccountingCustomerParty>
    <Party>
      <PartyIdentification>
        <ID></ID>
      </PartyIdentification>
      <PartyName>
        <Name>Jan Novák</Name>
      </PartyName>
      <PostalAddress>
        <StreetName>Krátká 3</StreetName>
        <BuildingNumber></BuildingNumber>
        <CityName>Brno</CityName>
        <PostalZone>602 00</PostalZone>
        <Country>
          <IdentificationCode>CZ</IdentificationCode>
          <Name>Česká republika</Name>
        </Country>
      </PostalAddress>
      <Contact>
        <ElectronicMail>jan@example.cz</ElectronicMail>
      </Contact>
    </Party>
  </AccountingCustomerParty>
  <OrderReferences>
    <OrderReference>
      <SalesOrderID>OBJ-2026-0042</SalesOrderID>
    </OrderReference>
  </OrderReferences>
  <InvoiceLines>
    <InvoiceLine>
      <ID>1</ID>
      <InvoicedQuantity>1</InvoicedQuantity>
      <LineExtensionAmount>2490.00</LineExtensionAmount>
      <LineExtensionAmountTaxInclusive>2490.00</LineExtensionAmountTaxInclusive>
      <LineExtensionTaxAmount>0.00</LineExtensionTaxAmount>
      <UnitPrice>2490.00</UnitPrice>
      <UnitPriceTaxInclusive>2490.00</UnitPriceTaxInclusive>
      <ClassifiedTaxCategory>
        <Percent>0</Percent>
        <VATCalculationMethod>0</VATCalculationMethod>
        <VATApplicable>false</VATApplicable>
      </ClassifiedTaxCategory>
      <Item>
        <Description>Ručně šitá kožená taška&#xA;hnědá / 30 cm</Description>
      </Item>
    </InvoiceLine>
    <InvoiceLine>
      <ID>2</ID>
      <InvoicedQuantity>2</InvoicedQuantity>
      <LineExtensionAmount>900.00</LineExtensionAmount>
      <LineExtensionAmountTaxInclusive>900.00</LineExtensionAmountTaxInclusive>
      <LineExtensionTaxAmount>0.00</LineExtensionTaxAmount>
      <UnitPrice>450.00</UnitPrice>
      <UnitPriceTaxInclusive>450.00</UnitPriceTaxInclusive>
      <ClassifiedTaxCategory>
        <Percent>0</Percent>
        <VATCalculationMethod>0</VATCalculationMethod>
        <VATApplicable>false</VATApplicable>
      </ClassifiedTaxCategory>
      <Item>
        <Description>Kožená peněženka</Description>
      </Item>
    </InvoiceLine>
  </InvoiceLines>
  <TaxTotal>
    <TaxSubTotal>
      <TaxableAmount>3460.00</TaxableAmount>
      <TaxAmount>0.00</TaxAmount>
      <TaxInclusiveAmount>3460.00</TaxInclusiveAmount>
      <AlreadyClaimedTaxableAmount>0.00</AlreadyClaimedTaxableAmount>
      <AlreadyClaimedTaxAmount>0.00</AlreadyClaimedTaxAmount>
      <AlreadyClaimedTaxInclusiveAmount>0.00</AlreadyClaimedTaxInclusiveAmount>
      <DifferenceTaxableAmount>3460.00</DifferenceTaxableAmount>
      <DifferenceTaxAmount>0.00</DifferenceTaxAmount>
      <DifferenceTaxInclusiveAmount>3460.00</DifferenceTaxInclusiveAmount>
      <TaxCategory>
        <Percent>0</Percent>
        <VATApplicable>false</VATApplicable>
      </TaxCategory>
    </TaxSubTotal>
    <TaxAmount>0.00</TaxAmount>
  </TaxTotal>
  <LegalMonetaryTotal>
    <TaxExclusiveAmount>3460.00</TaxExclusiveAmount>
    <TaxInclusiveAmount>3460.00</TaxInclusiveAmount>
    <AlreadyClaimedTaxExclusiveAmount>0.00</AlreadyClaimedTaxExclusiveAmount>
    <AlreadyClaimedTaxInclusiveAmount>0.00</AlreadyClaimedTaxInclusiveAmount>
    <DifferenceTaxExclusiveAmount>3460.00</DifferenceTaxExclusiveAmount>
    <DifferenceTaxInclusiveAmount>3460.00</DifferenceTaxInclusiveAmount>
    <PaidDepositsAmount>0.00</PaidDepositsAmount>
    <PayableAmount>3460.00</PayableAmount>
  </LegalMonetaryTotal>
</Invoice>
`

func goldenDocument() invoicepdf.Document {
	return invoicepdf.Document{
		Number:        "2026-0042",
		Kind:          invoicepdf.KindInvoice,
		IssuedDate:    "2026-09-19",
		SupplyDate:    "2026-09-18",
		OrderNumber:   "OBJ-2026-0042",
		Locale:        "cs-CZ",
		Currency:      "CZK",
		SellerName:    "Sample Goods s.r.o.",
		SellerID:      "12345678",
		SellerAddress: "Dlouhá 12\n110 00 Praha 1",
		SellerEmail:   "billing@example.com",
		SellerPhone:   "+420 123 456 789",
		BuyerName:     "Jan Novák",
		BuyerAddress:  "Krátká 3\n602 00 Brno",
		BuyerEmail:    "jan@example.cz",
		Lines: []invoicepdf.Line{
			{
				Name:           "Ručně šitá kožená taška",
				Configuration:  "hnědá / 30 cm",
				Quantity:       1,
				UnitPriceMinor: 249000,
				TotalMinor:     249000,
			},
			{
				Name:           "Kožená peněženka",
				Quantity:       2,
				UnitPriceMinor: 45000,
				TotalMinor:     90000,
			},
		},
		ItemsMinor:    339000,
		ShippingMinor: 12000,
		DiscountMinor: 5000,
		TotalMinor:    346000,
		Paid:          true,
	}
}

// isdocProbe decodes the parts of the XML that several tests assert.
type isdocProbe struct {
	XMLName       xml.Name `xml:"Invoice"`
	DocumentType  int      `xml:"DocumentType"`
	VATApplicable bool     `xml:"VATApplicable"`
	Note          struct {
		LanguageID string `xml:"languageID,attr"`
		Text       string `xml:",chardata"`
	} `xml:"Note"`
	OriginalDocumentReferences struct {
		OriginalDocumentReference struct {
			ID string `xml:"ID"`
		} `xml:"OriginalDocumentReference"`
	} `xml:"OriginalDocumentReferences"`
	InvoiceLines struct {
		InvoiceLine []struct {
			LineExtensionAmount   string `xml:"LineExtensionAmount"`
			ClassifiedTaxCategory struct {
				VATApplicable bool `xml:"VATApplicable"`
			} `xml:"ClassifiedTaxCategory"`
		} `xml:"InvoiceLine"`
	} `xml:"InvoiceLines"`
	TaxTotal struct {
		TaxSubTotal struct {
			TaxAmount string `xml:"TaxAmount"`
		} `xml:"TaxSubTotal"`
	} `xml:"TaxTotal"`
	LegalMonetaryTotal struct {
		PayableAmount string `xml:"PayableAmount"`
	} `xml:"LegalMonetaryTotal"`
}

func decodeISDOC(t *testing.T, data []byte) isdocProbe {
	t.Helper()
	var probe isdocProbe
	if err := xml.Unmarshal(data, &probe); err != nil {
		t.Fatalf("decode isdoc: %v", err)
	}
	return probe
}

func TestRenderISDOCGoldenInvoice(t *testing.T) {
	t.Parallel()
	data, err := invoicepdf.RenderISDOC(goldenDocument())
	if err != nil {
		t.Fatalf("RenderISDOC: %v", err)
	}
	if string(data) != isdocInvoiceGolden {
		t.Fatalf("RenderISDOC output differs from the golden XML:\n%s", data)
	}
	second, err := invoicepdf.RenderISDOC(goldenDocument())
	if err != nil {
		t.Fatalf("RenderISDOC(second run): %v", err)
	}
	if !bytes.Equal(data, second) {
		t.Fatalf("RenderISDOC is not deterministic for an equal document")
	}
	probe := decodeISDOC(t, data)
	if probe.DocumentType != 1 || probe.VATApplicable {
		t.Fatalf("unexpected document type or VAT flag: %+v", probe)
	}
	if probe.LegalMonetaryTotal.PayableAmount != "3460.00" {
		t.Fatalf("PayableAmount = %q, want %q", probe.LegalMonetaryTotal.PayableAmount, "3460.00")
	}
	if count := strings.Count(string(data), "<TaxSubTotal>"); count != 1 {
		t.Fatalf("expected exactly one zero-rate TaxSubTotal, found %d", count)
	}
	for _, line := range probe.InvoiceLines.InvoiceLine {
		if line.ClassifiedTaxCategory.VATApplicable {
			t.Fatalf("line is marked as subject to VAT")
		}
	}
}

func TestRenderISDOCContainsNoShopBrandStrings(t *testing.T) {
	t.Parallel()
	data, err := invoicepdf.RenderISDOC(goldenDocument())
	if err != nil {
		t.Fatalf("RenderISDOC: %v", err)
	}
	for _, brand := range []string{"Barkelier", "barkelier"} {
		if bytes.Contains(data, []byte(brand)) {
			t.Fatalf("RenderISDOC output contains brand string %q", brand)
		}
	}
	if !bytes.Contains(data, []byte("<IssuingSystem>invoicepdf</IssuingSystem>")) {
		t.Fatalf("RenderISDOC output has no issuing system")
	}
}

func TestRenderISDOCCreditNoteNormalizesAmounts(t *testing.T) {
	t.Parallel()
	document := goldenDocument()
	document.Kind = invoicepdf.KindCreditNote
	document.OriginalNumber = "2026-0042"
	document.Reason = "Vrácení zboží.\nDruhý řádek."
	document.TotalMinor = -346000
	document.Paid = false

	data, err := invoicepdf.RenderISDOC(document)
	if err != nil {
		t.Fatalf("RenderISDOC(credit note): %v", err)
	}
	probe := decodeISDOC(t, data)
	if probe.DocumentType != 2 {
		t.Fatalf("DocumentType = %d, want 2", probe.DocumentType)
	}
	if probe.OriginalDocumentReferences.OriginalDocumentReference.ID != "2026-0042" {
		t.Fatalf("original document reference = %q", probe.OriginalDocumentReferences.OriginalDocumentReference.ID)
	}
	if probe.Note.LanguageID != "cs" || probe.Note.Text != "Vrácení zboží.\nDruhý řádek." {
		t.Fatalf("note = %+v", probe.Note)
	}
	if probe.LegalMonetaryTotal.PayableAmount != "3460.00" {
		t.Fatalf("PayableAmount = %q, want the positive credit note total", probe.LegalMonetaryTotal.PayableAmount)
	}
	for _, line := range probe.InvoiceLines.InvoiceLine {
		if strings.HasPrefix(line.LineExtensionAmount, "-") {
			t.Fatalf("credit note line amount %q is negative", line.LineExtensionAmount)
		}
		if line.ClassifiedTaxCategory.VATApplicable {
			t.Fatalf("credit note line is marked as subject to VAT")
		}
	}
}

func TestRenderISDOCRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(document *invoicepdf.Document)
	}{
		{"empty number", func(document *invoicepdf.Document) { document.Number = " " }},
		{"unknown kind", func(document *invoicepdf.Document) { document.Kind = "debit_note" }},
		{"empty issue date", func(document *invoicepdf.Document) { document.IssuedDate = "" }},
		{"broken issue date", func(document *invoicepdf.Document) { document.IssuedDate = "19.09.2026" }},
		{"empty seller", func(document *invoicepdf.Document) { document.SellerName = " " }},
		{"empty buyer", func(document *invoicepdf.Document) { document.BuyerName = "" }},
		{"empty currency", func(document *invoicepdf.Document) { document.Currency = "" }},
		{"short currency", func(document *invoicepdf.Document) { document.Currency = "CZ" }},
		{"no lines", func(document *invoicepdf.Document) { document.Lines = nil }},
		{"credit note without original", func(document *invoicepdf.Document) {
			document.Kind = invoicepdf.KindCreditNote
			document.OriginalNumber = ""
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			document := goldenDocument()
			testCase.mutate(&document)
			_, err := invoicepdf.RenderISDOC(document)
			if err == nil {
				t.Fatalf("RenderISDOC(%s) succeeded, want error", testCase.name)
			}
			if !errors.Is(err, invoicepdf.ErrInvalidISDOC) {
				t.Fatalf("RenderISDOC(%s) error = %v, want ErrInvalidISDOC", testCase.name, err)
			}
		})
	}
}

func TestRenderISDOCAcceptsMinimalDocument(t *testing.T) {
	t.Parallel()
	document := invoicepdf.Document{
		Number:     "2026-0001",
		IssuedDate: "2026-01-05",
		Currency:   "EUR",
		SellerName: "Sample Goods",
		BuyerName:  "Jan Novák",
		Lines: []invoicepdf.Line{{
			Name:     "Widget",
			Quantity: 1,
		}},
	}

	data, err := invoicepdf.RenderISDOC(document)
	if err != nil {
		t.Fatalf("RenderISDOC(minimal): %v", err)
	}
	probe := decodeISDOC(t, data)
	if probe.TaxTotal.TaxSubTotal.TaxAmount != "0.00" {
		t.Fatalf("TaxAmount = %q, want zero VAT", probe.TaxTotal.TaxSubTotal.TaxAmount)
	}
	if probe.LegalMonetaryTotal.PayableAmount != "0.00" {
		t.Fatalf("PayableAmount = %q, want zero", probe.LegalMonetaryTotal.PayableAmount)
	}
	text := string(data)
	for _, unexpected := range []string{"<Contact>", "<OrderReferences>", "<Note", "<OriginalDocumentReferences>"} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("minimal document contains %s", unexpected)
		}
	}
	for _, expected := range []string{
		"<StreetName></StreetName>",
		"<PostalZone></PostalZone>",
		"<BuildingNumber></BuildingNumber>",
		"<ElectronicPossibilityAgreementReference></ElectronicPossibilityAgreementReference>",
		"<LocalCurrencyCode>EUR</LocalCurrencyCode>",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("minimal document is missing %s", expected)
		}
	}
	if probe.DocumentType != 1 || probe.VATApplicable {
		t.Fatalf("unexpected document type or VAT flag: %+v", probe)
	}
}

func TestRenderISDOCPostalAddressHeuristics(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		address string
		street  string
		city    string
		postal  string
	}{
		{
			name:    "two lines",
			address: "Dlouhá 12\n110 00 Praha 1",
			street:  "Dlouhá 12",
			city:    "Praha 1",
			postal:  "110 00",
		},
		{
			name:    "one line with comma",
			address: "Dlouhá 12, 110 00 Praha 1",
			street:  "Dlouhá 12",
			city:    "Praha 1",
			postal:  "110 00",
		},
		{
			name:    "zone without space",
			address: "11000 Praha 1",
			street:  "",
			city:    "Praha 1",
			postal:  "110 00",
		},
		{
			name:    "zone line after street line",
			address: "Ulice 4\nDlouhá 12, 110 00 Praha 1",
			street:  "Ulice 4, Dlouhá 12",
			city:    "Praha 1",
			postal:  "110 00",
		},
		{
			name:    "no zone",
			address: "Dlouhá 12\nPraha",
			street:  "Dlouhá 12, Praha",
			city:    "",
			postal:  "",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			document := goldenDocument()
			document.SellerAddress = testCase.address
			document.BuyerAddress = ""
			data, err := invoicepdf.RenderISDOC(document)
			if err != nil {
				t.Fatalf("RenderISDOC: %v", err)
			}
			var probe struct {
				AccountingSupplierParty struct {
					Party struct {
						PostalAddress struct {
							StreetName string `xml:"StreetName"`
							CityName   string `xml:"CityName"`
							PostalZone string `xml:"PostalZone"`
						} `xml:"PostalAddress"`
					} `xml:"Party"`
				} `xml:"AccountingSupplierParty"`
			}
			if err := xml.Unmarshal(data, &probe); err != nil {
				t.Fatalf("decode isdoc: %v", err)
			}
			address := probe.AccountingSupplierParty.Party.PostalAddress
			if address.StreetName != testCase.street ||
				address.CityName != testCase.city ||
				address.PostalZone != testCase.postal {
				t.Fatalf(
					"address %q decoded as street %q city %q zone %q",
					testCase.address, address.StreetName, address.CityName, address.PostalZone,
				)
			}
		})
	}
}

func TestRenderISDOCAcceptsRFC3339IssuedDate(t *testing.T) {
	t.Parallel()
	document := goldenDocument()
	document.IssuedDate = "2026-09-19T10:20:30+02:00"
	data, err := invoicepdf.RenderISDOC(document)
	if err != nil {
		t.Fatalf("RenderISDOC(rfc3339): %v", err)
	}
	if !strings.Contains(string(data), "<IssueDate>2026-09-19</IssueDate>") {
		t.Fatalf("RFC 3339 issued date was not converted to an ISDOC date")
	}
}

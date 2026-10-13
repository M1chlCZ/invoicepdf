# invoicepdf

Render invoices and credit notes as PDF bytes. The package uses embedded fonts and needs no files, no configuration, and no external services.

## Features

- `Render` returns the complete PDF in one call and never changes the document.
- Long item names and configurations wrap across lines and pages. A continuation page repeats the table header.
- Credit notes support negative amounts, an original-document reference, and a correction reason.
- Czech and English output. The Czech locale uses `Faktura`, `Kč`, comma decimals with non-breaking-space grouping, and `02.01.2006` dates. The English locale uses `Invoice`, `1,234.56`, and `02 Jan 2006`.
- Amounts use minor units as `int64`. Money formatting uses integer arithmetic only.
- An optional logo, an optional color palette, and an optional payment QR image.
- The paid or unpaid banner and the "not a VAT payer" note are part of the layout.
- `RenderISDOC` writes the same document as deterministic ISDOC 6.0.2 XML. See [ISDOC export](#isdoc-export).
- `LookupLabels` exposes the Czech and English label set for mailers. See [Labels](#labels).
- A batch command renders JSON documents to PDF and ISDOC. See [Command line](#command-line).

## Install

```
go get github.com/m1chlcz/invoicepdf
```

Go 1.27 or later is required.

## Quick start

```go
document := invoicepdf.Document{
	Number:        "2026-0001",
	Kind:          invoicepdf.KindInvoice,
	IssuedDate:    "2026-09-19",
	SupplyDate:    "2026-09-18",
	Locale:        "cs-CZ",
	Currency:      "CZK",
	SellerName:    "Sample Goods s.r.o.",
	SellerAddress: "Ulice 12\n110 00 Praha 1",
	BuyerName:     "Jan Novák",
	BuyerAddress:  "Krátká 3\n602 00 Brno",
	Lines: []invoicepdf.Line{{
		Name:           "Kožená taška",
		Quantity:       1,
		UnitPriceMinor: 249000,
		TotalMinor:     249000,
	}},
	ItemsMinor:    249000,
	ShippingMinor: 12000,
	TotalMinor:    261000,
	Paid:          true,
}

pdf, err := invoicepdf.Render(document)
if err != nil {
	return err
}
```

## Document fields

| Field | Meaning |
| --- | --- |
| `Number` | Document number. It appears in the header and in the PDF title. |
| `Kind` | `KindInvoice` or `KindCreditNote`. |
| `OriginalNumber` | The corrected document. Credit notes only. |
| `IssuedDate`, `SupplyDate` | ISO dates (`2006-01-02`) or RFC 3339. |
| `OrderNumber` | Optional order reference. |
| `Locale` | A `cs` prefix selects Czech. Any other value selects English. |
| `Currency` | An ISO code. `CZK` becomes `Kč` in Czech. |
| `SellerName`, `SellerID`, `SellerAddress`, `SellerEmail`, `SellerPhone` | Seller details. `SellerID` is the company ID. |
| `BuyerName`, `BuyerAddress`, `BuyerEmail` | Buyer details. |
| `Lines` | Line items with `Name`, `Configuration`, `Quantity`, `UnitPriceMinor`, and `TotalMinor`. |
| `ItemsMinor`, `ShippingMinor`, `DiscountMinor`, `TotalMinor` | Amounts in minor units. |
| `Reason` | The correction reason. Credit notes only. |
| `Paid` | Shows the paid banner and the settled total label. A credit note always uses the settled label and has no banner. |
| `Logo` | Optional PNG or JPEG bytes. The header keeps a 3:1 ratio. |
| `PaymentQR` | Optional payment QR (`PaymentQR{PNG, Label}`). |
| `Palette` | Optional color override. |

## Credit notes

Set `Kind` to `KindCreditNote`, set `OriginalNumber`, and use negative amounts. The renderer adds the original-document reference and the correction reason.

```go
document.Kind = invoicepdf.KindCreditNote
document.OriginalNumber = "2026-0001"
document.Reason = "Returned goods."
```

## Logo and colors

`Logo` accepts PNG or JPEG bytes. The renderer draws the logo in the header and skips the logo when the value is empty.

`Palette` replaces the built-in colors as a whole. Set every color that the document needs. An unset field is black.

```go
document.Palette = &invoicepdf.Palette{
	Primary: invoicepdf.Color{R: 0x1F, G: 0x29, B: 0x37},
	Surface: invoicepdf.Color{R: 0xF1, G: 0xF5, B: 0xF9},
	Accent:  invoicepdf.Color{R: 0x47, G: 0x55, B: 0x69},
	Text:    invoicepdf.Color{R: 0x11, G: 0x18, B: 0x27},
	Muted:   invoicepdf.Color{R: 0x6B, G: 0x72, B: 0x80},
	Rule:    invoicepdf.Color{R: 0xE2, G: 0xE8, B: 0xF0},
	RowAlt:  invoicepdf.Color{R: 0xF8, G: 0xFA, B: 0xFC},
	White:   invoicepdf.Color{R: 0xFF, G: 0xFF, B: 0xFF},
}
```

## Payment QR

`PaymentQR` embeds a payment QR PNG next to the totals and prints its label
under the image. The caller generates the PNG; the package only places it.
Generate one with [qrplatba](https://github.com/m1chlcz/qrplatba):

```
qrplatba --iban CZ6508000000192000145399 --amount 2560.00 --vs 20260042 \
  --message "Faktura 2026-0042" --output qr.png
```

```go
document.PaymentQR = &invoicepdf.PaymentQR{
	PNG:   qrPNG,
	Label: "QR Platba",
}
```

The image renders 96 points wide. An empty `Label` draws no caption. `Render`
returns an error when `PNG` is empty or not a valid PNG.

## ISDOC export

`RenderISDOC` returns the same document as ISDOC 6.0.2 XML in the
`http://isdoc.cz/namespace/2013` name space. The writer is deterministic:
equal documents return equal bytes. The standard is published at
[isdoc.cz](https://isdoc.cz/); the output validates against the official
`isdoc-invoice-6.0.2.xsd` schema.

```go
xmlBytes, err := invoicepdf.RenderISDOC(document)
if err != nil {
    return err
}
```

`RenderISDOC` reports `ErrInvalidISDOC` for a document the format cannot hold.
The document must carry a number, an issue date, a three-letter currency, a
seller, a buyer and at least one line; a credit note must carry
`OriginalNumber`.

### Field mapping

| Document | ISDOC | Notes |
| --- | --- | --- |
| `Number` | `ID` | Required. |
| `Kind` | `DocumentType` | `KindInvoice` or an empty kind is `1`, `KindCreditNote` is `2`. Any other kind is an error. |
| `OriginalNumber` | `OriginalDocumentReferences/OriginalDocumentReference/ID` | Required for credit notes. |
| `IssuedDate` | `IssueDate` | ISO date or RFC 3339; written as `YYYY-MM-DD`. |
| `SupplyDate` | - | Non-VAT documents carry no `TaxPointDate` (ISDOC rules 4.1.5 and A.5). |
| `OrderNumber` | `OrderReferences/OrderReference/SalesOrderID` | Optional. |
| `Locale` | `Note/@languageID` | `cs` or `en`; used on the credit-note reason. |
| `Currency` | `LocalCurrencyCode` | Required three-letter code. `CurrRate` and `RefCurrRate` are `1`; amounts are not converted. |
| `SellerName` | `AccountingSupplierParty/Party/PartyName/Name` | Required. |
| `SellerID` | `AccountingSupplierParty/Party/PartyIdentification/ID` | Empty element when missing. |
| `SellerAddress` | `AccountingSupplierParty/Party/PostalAddress` | Deterministic split; see [Addresses](#addresses). |
| `SellerEmail`, `SellerPhone` | `AccountingSupplierParty/Party/Contact/ElectronicMail`, `.../Telephone` | `Contact` is omitted when both are empty. |
| `BuyerName` | `AccountingCustomerParty/Party/PartyName/Name` | Required. |
| `BuyerAddress` | `AccountingCustomerParty/Party/PostalAddress` | Same rules. The buyer ID is always an empty element; the model has none. |
| `BuyerEmail` | `AccountingCustomerParty/Party/Contact/ElectronicMail` | |
| `Line.Name`, `Line.Configuration` | `InvoiceLines/InvoiceLine/Item/Description` | Joined with a newline. |
| `Line.Quantity` | `InvoicedQuantity` | No unit code; the model has no units. |
| `Line.UnitPriceMinor` | `UnitPrice`, `UnitPriceTaxInclusive` | Equal; VAT is zero. |
| `Line.TotalMinor` | `LineExtensionAmount`, `LineExtensionAmountTaxInclusive` | Equal; `LineExtensionTaxAmount` is `0.00`. |
| `ItemsMinor`, `ShippingMinor`, `DiscountMinor`, `TotalMinor` | `TaxTotal`, `LegalMonetaryTotal` | Only `TotalMinor` is written. ISDOC 6.0.2 has no document-level allowance element, so shipping and discount are included in the total. |
| `Reason` | `Note` | Credit notes only. |
| Document identity | `UUID` | Deterministic RFC 4122 version 5 UUID from the number, kind, issue date, seller, original number and total. |
| `Paid`, `PaymentQR`, `Logo`, `Palette` | - | Not part of ISDOC. Payment means and bank details cannot be expressed by the model. |

Derived values: `IssuingSystem` is `invoicepdf`,
`ElectronicPossibilityAgreementReference` is empty (required by the schema),
and every amount is written with two decimal places in minor units.

### Addresses

`PostalAddress` needs street, building number, city, postal zone and country.
The model stores one free-form address string, so the writer splits it
deterministically:

- Lines are trimmed; empty lines are dropped; `StreetName` joins the
  remaining lines with `, `.
- When the last line is `NNN NN City` or `NNN City` (for example
  `110 00 Praha 1`), the zone and city are split off.
- When the last line is `Street, NNN NN City` (for example
  `Dlouhá 12, 110 00 Praha 1`), the part before the comma joins the street.
- `BuildingNumber` stays empty; the building number remains part of
  `StreetName`.
- `Country` is always `CZ` and `Česká republika`; the model has no country.

### Limits

- The model has no VAT rates, VAT IDs or tax amounts. Every document is
  written as a non-VAT document: `VATApplicable` is `false`, every line has a
  zero percent tax category with `VATApplicable` `false`, and one zero-rate
  `TaxSubTotal` reconciles the total. Multiple VAT rates are not expressible.
- Credit notes are written with positive amounts, as ISDOC rule A.6 requires;
  `DocumentType` `2` carries the correction. The PDF keeps the negative
  amounts.
- ISDOC requires at least one line, so an empty `Lines` slice is an error.
- `PaymentMeans` is omitted because the model carries no payment details.

## Labels

`LookupLabels` returns the label set `Render` draws, so a mailer can reuse the
same strings:

```go
labels, err := invoicepdf.LookupLabels(document.Locale)
if err != nil {
    return err
}
notice := labels.PaidNotice
```

A locale with the `cs` prefix selects Czech, the `en` prefix or an empty
locale selects English. Any other locale reports `ErrUnsupportedLocale`;
`Render` itself falls back to English for unknown locales.

## Command line

```
go install github.com/m1chlcz/invoicepdf/cmd/invoicepdf@latest
invoicepdf render --out-dir DIR [--isdoc] INPUT...
```

An `INPUT` is a JSON document file or a directory of `*.json` files.
Directories are read in name order; an explicit list is processed in the given
order. Every input writes `DIR/<name>.pdf` and, with `--isdoc`,
`DIR/<name>.isdoc`. Per-file status goes to stderr. A deterministic JSON
summary goes to stdout:

```json
{"version":1,"total":2,"succeeded":1,"failed":1,"results":[
  {"input":"in/a.json","pdf":"out/a.pdf","isdoc":"out/a.isdoc","ok":true,"error":null},
  {"input":"in/b.json","pdf":"","isdoc":"","ok":false,"error":"decode document: ..."}
]}
```

Exit codes: `0` when every document renders, `1` when any document fails, `2`
for usage errors (unknown command, missing `--out-dir`, missing input, and so
on).

The input JSON mirrors `Document`. Unknown fields are rejected so typos fail
loudly. All fields are optional; a document that `Render` accepts still
renders. With `--isdoc`, the stricter `RenderISDOC` rules decide the status of
the same file.

| JSON field | Type | Document field |
| --- | --- | --- |
| `number`, `kind`, `original_number`, `issued_date`, `supply_date`, `order_number`, `locale`, `currency` | string | `Number`, `Kind`, `OriginalNumber`, `IssuedDate`, `SupplyDate`, `OrderNumber`, `Locale`, `Currency` |
| `seller_name`, `seller_id`, `seller_address`, `seller_email`, `seller_phone` | string | `SellerName`, `SellerID`, `SellerAddress`, `SellerEmail`, `SellerPhone` |
| `buyer_name`, `buyer_address`, `buyer_email` | string | `BuyerName`, `BuyerAddress`, `BuyerEmail` |
| `logo_base64` | base64 string | `Logo` (PNG or JPEG) |
| `payment_qr` | `{"png_base64": string, "label": string}` | `PaymentQR` |
| `palette` | `{"primary", "surface", "accent", "text", "muted", "rule", "row_alt", "white"}` as `#RRGGBB` | `Palette` |
| `lines` | array of `{"name", "configuration", "quantity", "unit_price_minor", "total_minor"}` | `Lines` |
| `items_minor`, `shipping_minor`, `discount_minor`, `total_minor` | integer minor units | `ItemsMinor`, `ShippingMinor`, `DiscountMinor`, `TotalMinor` |
| `reason` | string | `Reason` |
| `paid` | bool | `Paid` |

## Development

```
go test ./...
```

Set `INVOICEPDF_SAMPLE_DIR` to an absolute path to export sample PDFs:

```
INVOICEPDF_SAMPLE_DIR=/tmp/invoicepdf go test -run TestExportSamples -v
```

## License

MIT. See `LICENSE`.

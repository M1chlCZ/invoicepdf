# invoicepdf

Render invoices and credit notes as PDF bytes. The package uses embedded fonts and needs no files, no configuration, and no external services.

## Features

- `Render` returns the complete PDF in one call and never changes the document.
- Long item names and configurations wrap across lines and pages. A continuation page repeats the table header.
- Credit notes support negative amounts, an original-document reference, and a correction reason.
- Czech and English output. The Czech locale uses `Faktura`, `Kč`, comma decimals with non-breaking-space grouping, and `02.01.2006` dates. The English locale uses `Invoice`, `1,234.56`, and `02 Jan 2006`.
- Amounts use minor units as `int64`. Money formatting uses integer arithmetic only.
- An optional logo and an optional color palette.
- The paid or unpaid banner and the "not a VAT payer" note are part of the layout.

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

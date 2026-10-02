package invoicepdf_test

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m1chlcz/invoicepdf"
)

const (
	sampleSellerName    = "Sample Goods s.r.o."
	sampleSellerID      = "12345678"
	sampleSellerAddress = "Ulice 12\n110 00 Praha 1"
	sampleSellerEmail   = "billing@example.com"
	sampleSellerPhone   = "+420 123 456 789"
)

func TestRenderReturnsCompleteSnapshot(t *testing.T) {
	t.Parallel()
	document := invoicepdf.Document{
		Number:        "2026-0001",
		Kind:          invoicepdf.KindInvoice,
		IssuedDate:    "2026-09-19",
		SupplyDate:    "2026-09-18",
		OrderNumber:   "OBJ-42",
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
		},
		ItemsMinor:    249000,
		ShippingMinor: 12000,
		DiscountMinor: 5000,
		TotalMinor:    256000,
		Paid:          true,
	}

	data, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("Render(invoice): %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("rendered bytes are not a PDF: %q", data[:min(8, len(data))])
	}
	if !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatalf("rendered PDF has no EOF marker")
	}
	if len(data) < 1000 {
		t.Fatalf("rendered PDF unexpectedly small: %d bytes", len(data))
	}
}

func TestRenderCreditNoteWrapsLongNamesAcrossPages(t *testing.T) {
	t.Parallel()
	longName := strings.Repeat("Ručně šitá kožená aktovka s vyztuženou přihrádkou ", 12) +
		"Příliš žluťoučký kůň úpěl ďábelské ódy"
	lines := make([]invoicepdf.Line, 0, 60)
	for index := range 60 {
		lines = append(lines, invoicepdf.Line{
			Name:           fmt.Sprintf("%s č. %d", longName, index+1),
			Configuration:  strings.Repeat("vnitřní podšívka / mosazné kování / ", 8),
			Quantity:       int64(index + 1),
			UnitPriceMinor: -129900,
			TotalMinor:     -129900 * int64(index+1),
		})
	}

	document := invoicepdf.Document{
		Number:         "2026-0007",
		Kind:           invoicepdf.KindCreditNote,
		OriginalNumber: "2026-0003",
		IssuedDate:     "2026-09-19",
		SupplyDate:     "2026-09-15",
		Locale:         "cs-CZ",
		Currency:       "CZK",
		SellerName:     "Sample Goods s.r.o.",
		SellerID:       "12345678",
		SellerAddress:  "Dlouhá 12\n110 00 Praha 1",
		SellerEmail:    "billing@example.com",
		BuyerName:      "Společnost s velmi dlouhým názvem, a.s.",
		BuyerAddress:   "Nějaká opravdu dlouhá adresa 1234/56\n602 00 Brno",
		BuyerEmail:     "jan@example.cz",
		Lines:          lines,
		ItemsMinor:     -7794000,
		TotalMinor:     -7794000,
		Reason: strings.Repeat(
			"Reklamace a vrácení zboží z důvodu poškození při přepravě. ",
			6,
		),
		Paid: true,
	}

	data, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("Render(credit note): %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("rendered bytes are not a PDF")
	}
	if pages := bytes.Count(data, []byte("/Type /Page\n")); pages < 2 {
		t.Fatalf("expected a multipage credit note, rendered %d page(s)", pages)
	}
}

func TestRenderSplitsSingleOversizedLineAcrossPages(t *testing.T) {
	t.Parallel()
	document := invoicepdf.Document{
		Number:        "2026-0009",
		Kind:          invoicepdf.KindInvoice,
		IssuedDate:    "2026-09-19",
		Locale:        "cs-CZ",
		Currency:      "CZK",
		SellerName:    "Sample Goods",
		SellerAddress: "Praha",
		BuyerName:     "Jan Novák",
		BuyerAddress:  "Brno",
		Lines: []invoicepdf.Line{{
			Name:           strings.Repeat("Velmi dlouhý název položky ", 400),
			Quantity:       1,
			UnitPriceMinor: 100,
			TotalMinor:     100,
		}},
		ItemsMinor: 100,
		TotalMinor: 100,
	}

	data, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("Render(oversized line): %v", err)
	}
	if pages := bytes.Count(data, []byte("/Type /Page\n")); pages < 3 {
		t.Fatalf("expected an oversized line to span pages, rendered %d page(s)", pages)
	}
}

func TestRenderAcceptsCustomPaletteAndLogo(t *testing.T) {
	t.Parallel()
	var logo bytes.Buffer
	if err := png.Encode(&logo, image.NewRGBA(image.Rect(0, 0, 30, 10))); err != nil {
		t.Fatalf("encode logo: %v", err)
	}
	document := invoicepdf.Document{
		Number:     "2026-0011",
		Kind:       invoicepdf.KindInvoice,
		IssuedDate: "2026-09-19",
		Locale:     "en",
		Currency:   "EUR",
		SellerName: "Sample Goods s.r.o.",
		BuyerName:  "Jan Novák",
		Logo:       logo.Bytes(),
		Palette: &invoicepdf.Palette{
			Primary: invoicepdf.Color{0x30, 0x30, 0x30},
			Surface: invoicepdf.Color{0xEE, 0xEE, 0xEE},
		},
	}

	data, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("Render(custom palette, logo): %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("rendered bytes are not a PDF")
	}
}

func TestRenderRejectsInvalidLogo(t *testing.T) {
	t.Parallel()
	document := invoicepdf.Document{
		Number:     "2026-0012",
		Kind:       invoicepdf.KindInvoice,
		IssuedDate: "2026-09-19",
		Locale:     "en",
		Logo:       []byte("not an image"),
	}

	if _, err := invoicepdf.Render(document); err == nil {
		t.Fatalf("Render(invalid logo) succeeded, want error")
	}
}

const sampleDirEnv = "INVOICEPDF_SAMPLE_DIR"

// TestExportSamples writes review PDFs when the operator points sampleDirEnv
// at a writable absolute path. The test is skipped when the variable is empty.
func TestExportSamples(t *testing.T) {
	t.Parallel()
	directory := strings.TrimSpace(os.Getenv(sampleDirEnv))
	if directory == "" {
		t.Skip("set " + sampleDirEnv + " to export sample PDFs")
	}
	if !filepath.IsAbs(directory) {
		t.Fatalf("%s must be an absolute path outside the source tree, got %q", sampleDirEnv, directory)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create sample directory: %v", err)
	}

	itemsMinor := int64(249000 + 189000)
	shippingMinor := int64(12000)
	discountMinor := int64(40000)
	totalMinor := itemsMinor + shippingMinor - discountMinor

	paidInvoice := invoicepdf.Document{
		Number:        "2026-0042",
		Kind:          invoicepdf.KindInvoice,
		IssuedDate:    "2026-09-19",
		SupplyDate:    "2026-09-18",
		OrderNumber:   "OBJ-2026-0042",
		Locale:        "cs-CZ",
		Currency:      "CZK",
		SellerName:    sampleSellerName,
		SellerID:      sampleSellerID,
		SellerAddress: sampleSellerAddress,
		SellerEmail:   sampleSellerEmail,
		SellerPhone:   sampleSellerPhone,
		BuyerName:     "Jan Novák",
		BuyerAddress:  "Krátká 1234/56\n602 00 Brno",
		BuyerEmail:    "jan.novak@example.cz",
		ItemsMinor:    itemsMinor,
		ShippingMinor: shippingMinor,
		DiscountMinor: discountMinor,
		TotalMinor:    totalMinor,
		Lines: []invoicepdf.Line{
			{
				Name:           "Ručně šitá kožená taška",
				Configuration:  "hnědá / 30 cm",
				Quantity:       1,
				UnitPriceMinor: 249000,
				TotalMinor:     249000,
			},
			{
				Name:           "Kožená peněženka s ražbou",
				Configuration:  "černá / iniciály J. N.",
				Quantity:       1,
				UnitPriceMinor: 189000,
				TotalMinor:     189000,
			},
		},
		Paid: true,
	}
	writeSample(t, directory, "invoice-paid-cs.pdf", paidInvoice)

	creditLines := make([]invoicepdf.Line, 0, 40)
	var creditTotal int64
	for index := range 40 {
		lineTotal := int64(-129900 * (index + 1))
		creditTotal += lineTotal
		creditLines = append(creditLines, invoicepdf.Line{
			Name: fmt.Sprintf("Ručně šitá kožená aktovka s vyztuženou přihrádkou, %s č. %d",
				strings.Repeat("limitovaná edice ", 6), index+1),
			Configuration:  strings.Repeat("vnitřní podšívka / mosazné kování / ", 6),
			Quantity:       int64(index + 1),
			UnitPriceMinor: -129900,
			TotalMinor:     lineTotal,
		})
	}
	multipageCredit := invoicepdf.Document{
		Number:         "2026-0043",
		Kind:           invoicepdf.KindCreditNote,
		OriginalNumber: "2026-0042",
		IssuedDate:     "2026-09-19",
		SupplyDate:     "2026-09-18",
		OrderNumber:    "OBJ-2026-0042",
		Locale:         "cs-CZ",
		Currency:       "CZK",
		SellerName:     sampleSellerName,
		SellerID:       sampleSellerID,
		SellerAddress:  sampleSellerAddress,
		SellerEmail:    sampleSellerEmail,
		SellerPhone:    sampleSellerPhone,
		BuyerName:      "Společnost s velmi dlouhým obchodním názvem, a.s.",
		BuyerAddress:   "Opravdu dlouhá adresa 1234/56\n602 00 Brno",
		BuyerEmail:     "ucetni.oddeleni@example.cz",
		Lines:          creditLines,
		ItemsMinor:     creditTotal,
		TotalMinor:     creditTotal,
		Reason: strings.Repeat(
			"Reklamace a vrácení zboží z důvodu poškození při přepravě. ",
			10,
		),
		Paid: false,
	}
	writeSample(t, directory, "credit-note-multipage-cs.pdf", multipageCredit)
}

func writeSample(t *testing.T, directory, name string, document invoicepdf.Document) {
	t.Helper()
	if document.ItemsMinor+document.ShippingMinor-document.DiscountMinor != document.TotalMinor {
		t.Fatalf("sample %s has unreconciled totals", name)
	}
	data, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("render sample %s: %v", name, err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("sample %s is not a PDF", name)
	}
	path := filepath.Join(directory, name)
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
		t.Fatalf("write sample %s: %v", path, writeErr)
	}
	t.Logf("wrote %s (%d bytes)", path, len(data))
}

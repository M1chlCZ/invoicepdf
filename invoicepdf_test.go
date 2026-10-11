package invoicepdf_test

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

func sampleInvoice() invoicepdf.Document {
	return invoicepdf.Document{
		Number:        "2026-0021",
		Kind:          invoicepdf.KindInvoice,
		IssuedDate:    "2026-09-19",
		Locale:        "cs-CZ",
		Currency:      "CZK",
		SellerName:    "Sample Goods s.r.o.",
		SellerAddress: "Dlouhá 12\n110 00 Praha 1",
		BuyerName:     "Jan Novák",
		BuyerAddress:  "Krátká 3\n602 00 Brno",
		Lines: []invoicepdf.Line{{
			Name:           "Kožená taška",
			Quantity:       1,
			UnitPriceMinor: 249000,
			TotalMinor:     249000,
		}},
		ItemsMinor: 249000,
		TotalMinor: 249000,
	}
}

func sampleQRPNG(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			dark := (x+y)%2 == 0 || x < 2 || y < 2 || x >= size-2 || y >= size-2
			if dark {
				img.SetGray(x, y, color.Gray{Y: 0})
				continue
			}
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode payment qr: %v", err)
	}
	return buffer.Bytes()
}

// inflateDrawStreams returns the decoded page content that carries image draw
// operators. It ignores compressed font and image streams.
func inflateDrawStreams(t *testing.T, data []byte) string {
	t.Helper()
	var decoded strings.Builder
	for {
		start := bytes.Index(data, []byte("stream\n"))
		if start < 0 {
			break
		}
		data = data[start+len("stream\n"):]
		end := bytes.Index(data, []byte("\nendstream"))
		if end < 0 {
			break
		}
		raw := data[:end]
		data = data[end+len("\nendstream"):]
		reader, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			continue
		}
		content, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil || !bytes.Contains(content, []byte(" cm /I")) {
			continue
		}
		decoded.Write(content)
		decoded.WriteByte('\n')
	}
	return decoded.String()
}

var cmapEntryPattern = regexp.MustCompile(`<[0-9A-F]{4,6}><[0-9A-F]{4,6}><([0-9A-F]{4,6})>`)

// cmapCodePoints returns the Unicode code points of every character in the
// embedded font maps. Tests assert rendered text through these code points.
func cmapCodePoints(t *testing.T, data []byte) map[rune]bool {
	t.Helper()
	points := make(map[rune]bool)
	for _, match := range cmapEntryPattern.FindAllSubmatch(data, -1) {
		value, err := strconv.ParseUint(string(match[1]), 16, 32)
		if err != nil {
			t.Fatalf("parse font map code point %q: %v", match[1], err)
		}
		points[rune(value)] = true
	}
	return points
}

func TestRenderPaymentQRImageAndLabel(t *testing.T) {
	t.Parallel()
	document := sampleInvoice()
	document.PaymentQR = &invoicepdf.PaymentQR{
		PNG:   sampleQRPNG(t, 21),
		Label: "QR Platba",
	}

	data, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("Render(payment qr): %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("rendered bytes are not a PDF")
	}
	if count := bytes.Count(data, []byte("/Subtype /Image")); count != 1 {
		t.Fatalf("expected one payment qr image object, found %d", count)
	}
	if !bytes.Contains(data, []byte("/Width 21\n")) {
		t.Fatalf("embedded payment qr image has unexpected dimensions")
	}
	content := inflateDrawStreams(t, data)
	if !strings.Contains(content, "96.00 0 0") {
		t.Fatalf("payment qr is not drawn at the default width")
	}
	if draws := strings.Count(content, " Do "); draws != 1 {
		t.Fatalf("expected one payment qr draw operator, found %d", draws)
	}
	points := cmapCodePoints(t, data)
	for _, r := range "QR" {
		if !points[r] {
			t.Fatalf("payment qr label rune %q missing from the rendered font map", r)
		}
	}

	withoutLabel := sampleInvoice()
	withoutLabel.PaymentQR = &invoicepdf.PaymentQR{PNG: sampleQRPNG(t, 21)}
	plain, err := invoicepdf.Render(withoutLabel)
	if err != nil {
		t.Fatalf("Render(payment qr without label): %v", err)
	}
	if cmapCodePoints(t, plain)['Q'] {
		t.Fatalf("label rune %q rendered without a label", 'Q')
	}
}

func TestRenderRejectsInvalidPaymentQR(t *testing.T) {
	t.Parallel()
	document := sampleInvoice()
	document.PaymentQR = &invoicepdf.PaymentQR{PNG: []byte("not a png")}

	_, err := invoicepdf.Render(document)
	if err == nil {
		t.Fatalf("Render(invalid payment qr) succeeded, want error")
	}
	if !strings.Contains(err.Error(), "payment qr") || !strings.Contains(err.Error(), "invalid png") {
		t.Fatalf("Render(invalid payment qr) error = %q, want a descriptive png error", err)
	}

	empty := sampleInvoice()
	empty.PaymentQR = &invoicepdf.PaymentQR{}
	if _, err = invoicepdf.Render(empty); err == nil {
		t.Fatalf("Render(empty payment qr) succeeded, want error")
	}
}

func TestRenderPaymentQRCoexistsWithLogo(t *testing.T) {
	t.Parallel()
	var logo bytes.Buffer
	if err := png.Encode(&logo, image.NewGray(image.Rect(0, 0, 30, 10))); err != nil {
		t.Fatalf("encode logo: %v", err)
	}
	document := sampleInvoice()
	document.Logo = logo.Bytes()
	document.PaymentQR = &invoicepdf.PaymentQR{
		PNG:   sampleQRPNG(t, 21),
		Label: "QR Platba",
	}

	data, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("Render(logo and payment qr): %v", err)
	}
	if count := bytes.Count(data, []byte("/Subtype /Image")); count != 2 {
		t.Fatalf("expected logo and payment qr image objects, found %d", count)
	}
	if !bytes.Contains(data, []byte("/Width 30\n")) || !bytes.Contains(data, []byte("/Width 21\n")) {
		t.Fatalf("embedded logo and payment qr dimensions are not both present")
	}
	if draws := strings.Count(inflateDrawStreams(t, data), " Do "); draws != 2 {
		t.Fatalf("expected two image draw operators, found %d", draws)
	}
}

func TestRenderPaymentQRIsDeterministic(t *testing.T) {
	t.Parallel()
	document := sampleInvoice()
	document.PaymentQR = &invoicepdf.PaymentQR{
		PNG:   sampleQRPNG(t, 21),
		Label: "QR Platba",
	}

	first, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("Render(payment qr): %v", err)
	}
	second, err := invoicepdf.Render(document)
	if err != nil {
		t.Fatalf("Render(payment qr again): %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("Render returned different bytes for identical payment qr input")
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

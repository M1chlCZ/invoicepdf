package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/m1chlcz/invoicepdf"
)

const (
	exitOK          = 0
	exitFailed      = 1
	exitUsage       = 2
	summaryVersion  = 1
	jsonExtension   = ".json"
	pdfExtension    = ".pdf"
	isdocExtension  = ".isdoc"
	outputFileMode  = 0o600
	outputDirMode   = 0o755
	defaultBaseName = "document"
)

const usageText = `usage: invoicepdf render --out-dir DIR [--isdoc] INPUT...

INPUT is a JSON document file or a directory of *.json files. Every input
renders DIR/<name>.pdf and, with --isdoc, DIR/<name>.isdoc. Progress is
reported on stderr and a JSON summary is printed on stdout.

input fields:
  number, kind, original_number, issued_date, supply_date, order_number,
  locale, currency, seller_name, seller_id, seller_address, seller_email,
  seller_phone, buyer_name, buyer_address, buyer_email, reason, paid,
  logo_base64, payment_qr{png_base64,label}, palette{primary,surface,accent,
  text,muted,rule,row_alt,white}, lines[{name,configuration,quantity,
  unit_price_minor,total_minor}], items_minor, shipping_minor, discount_minor,
  total_minor
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return exitUsage
	}
	switch args[0] {
	case "render":
		return runRender(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "invoicepdf: unknown command %q\n", args[0])
		fmt.Fprint(stderr, usageText)
		return exitUsage
	}
}

func runRender(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("invoicepdf render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usageText) }
	outDir := flags.String("out-dir", "", "output directory for rendered documents")
	isdoc := flags.Bool("isdoc", false, "also write ISDOC 6.0.2 XML")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*outDir) == "" || flags.NArg() == 0 {
		flags.Usage()
		return exitUsage
	}
	inputs, err := collectInputs(flags.Args())
	if err != nil {
		fmt.Fprintf(stderr, "invoicepdf: %v\n", err)
		return exitUsage
	}
	if err = os.MkdirAll(*outDir, outputDirMode); err != nil {
		fmt.Fprintf(stderr, "invoicepdf: create out-dir: %v\n", err)
		return exitUsage
	}

	summary := runBatch(inputs, *outDir, *isdoc, stderr)
	encoded, err := json.Marshal(summary)
	if err != nil {
		fmt.Fprintf(stderr, "invoicepdf: encode summary: %v\n", err)
		return exitUsage
	}
	fmt.Fprintln(stdout, string(encoded))
	if summary.Failed > 0 {
		return exitFailed
	}
	return exitOK
}

func collectInputs(inputs []string) ([]string, error) {
	files := make([]string, 0, len(inputs))
	seen := make(map[string]bool)
	appendFile := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		files = append(files, path)
	}
	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", input, err)
		}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("input %s is not a regular file or directory", input)
			}
			appendFile(input)
			continue
		}
		entries, err := os.ReadDir(input)
		if err != nil {
			return nil, fmt.Errorf("read input directory %s: %w", input, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != jsonExtension {
				continue
			}
			appendFile(filepath.Join(input, entry.Name()))
		}
	}
	if len(files) == 0 {
		return nil, errors.New("no JSON input files found")
	}
	return files, nil
}

type renderSummary struct {
	Version   int            `json:"version"`
	Total     int            `json:"total"`
	Succeeded int            `json:"succeeded"`
	Failed    int            `json:"failed"`
	Results   []renderResult `json:"results"`
}

type renderResult struct {
	Input string  `json:"input"`
	PDF   string  `json:"pdf"`
	ISDOC string  `json:"isdoc"`
	OK    bool    `json:"ok"`
	Error *string `json:"error"`
}

func runBatch(files []string, outDir string, writeISDOC bool, stderr io.Writer) renderSummary {
	summary := renderSummary{
		Version: summaryVersion,
		Total:   len(files),
		Results: make([]renderResult, 0, len(files)),
	}
	used := make(map[string]bool)
	for index, input := range files {
		result := renderOne(input, outDir, writeISDOC, used)
		if result.OK {
			summary.Succeeded++
			fmt.Fprintf(stderr, "invoicepdf: [%d/%d] %s -> %s%s\n",
				index+1, len(files), input, result.PDF, isdocSuffix(result.ISDOC))
		} else {
			summary.Failed++
			fmt.Fprintf(stderr, "invoicepdf: [%d/%d] %s: failed: %s\n",
				index+1, len(files), input, *result.Error)
		}
		summary.Results = append(summary.Results, result)
	}
	return summary
}

func isdocSuffix(path string) string {
	if path == "" {
		return ""
	}
	return ", " + path
}

func renderOne(input, outDir string, writeISDOC bool, used map[string]bool) renderResult {
	result := renderResult{Input: input}
	fail := func(err error) renderResult {
		message := err.Error()
		result.Error = &message
		return result
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return fail(err)
	}
	document, err := decodeDocument(data)
	if err != nil {
		return fail(err)
	}
	pdf, err := invoicepdf.Render(document)
	if err != nil {
		return fail(err)
	}
	var isdoc []byte
	if writeISDOC {
		isdoc, err = invoicepdf.RenderISDOC(document)
		if err != nil {
			return fail(err)
		}
	}
	name := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	if name == "" || name == "." {
		name = defaultBaseName
	}
	if used[name] {
		return fail(fmt.Errorf("duplicate output name %q", name))
	}
	used[name] = true

	pdfPath := filepath.Join(outDir, name+pdfExtension)
	if err = os.WriteFile(pdfPath, pdf, outputFileMode); err != nil {
		return fail(err)
	}
	result.PDF = pdfPath
	if writeISDOC {
		isdocPath := filepath.Join(outDir, name+isdocExtension)
		if err = os.WriteFile(isdocPath, isdoc, outputFileMode); err != nil {
			return fail(err)
		}
		result.ISDOC = isdocPath
	}
	result.OK = true
	return result
}

type documentInput struct {
	Number         string          `json:"number"`
	Kind           string          `json:"kind"`
	OriginalNumber string          `json:"original_number"`
	IssuedDate     string          `json:"issued_date"`
	SupplyDate     string          `json:"supply_date"`
	OrderNumber    string          `json:"order_number"`
	Locale         string          `json:"locale"`
	Currency       string          `json:"currency"`
	SellerName     string          `json:"seller_name"`
	SellerID       string          `json:"seller_id"`
	SellerAddress  string          `json:"seller_address"`
	SellerEmail    string          `json:"seller_email"`
	SellerPhone    string          `json:"seller_phone"`
	BuyerName      string          `json:"buyer_name"`
	BuyerAddress   string          `json:"buyer_address"`
	BuyerEmail     string          `json:"buyer_email"`
	Logo           []byte          `json:"logo_base64"`
	PaymentQR      *paymentQRInput `json:"payment_qr"`
	Palette        *paletteInput   `json:"palette"`
	Lines          []lineInput     `json:"lines"`
	ItemsMinor     int64           `json:"items_minor"`
	ShippingMinor  int64           `json:"shipping_minor"`
	DiscountMinor  int64           `json:"discount_minor"`
	TotalMinor     int64           `json:"total_minor"`
	Reason         string          `json:"reason"`
	Paid           bool            `json:"paid"`
}

type lineInput struct {
	Name           string `json:"name"`
	Configuration  string `json:"configuration"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	TotalMinor     int64  `json:"total_minor"`
}

type paymentQRInput struct {
	PNG   []byte `json:"png_base64"`
	Label string `json:"label"`
}

type paletteInput struct {
	Primary string `json:"primary"`
	Surface string `json:"surface"`
	Accent  string `json:"accent"`
	Text    string `json:"text"`
	Muted   string `json:"muted"`
	Rule    string `json:"rule"`
	RowAlt  string `json:"row_alt"`
	White   string `json:"white"`
}

func decodeDocument(data []byte) (invoicepdf.Document, error) {
	var input documentInput
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return invoicepdf.Document{}, fmt.Errorf("decode document: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return invoicepdf.Document{}, errors.New("decode document: unexpected trailing data")
	}
	document, err := input.document()
	if err != nil {
		return invoicepdf.Document{}, err
	}
	return document, nil
}

func (input documentInput) document() (invoicepdf.Document, error) {
	document := invoicepdf.Document{
		Number:         input.Number,
		Kind:           input.Kind,
		OriginalNumber: input.OriginalNumber,
		IssuedDate:     input.IssuedDate,
		SupplyDate:     input.SupplyDate,
		OrderNumber:    input.OrderNumber,
		Locale:         input.Locale,
		Currency:       input.Currency,
		SellerName:     input.SellerName,
		SellerID:       input.SellerID,
		SellerAddress:  input.SellerAddress,
		SellerEmail:    input.SellerEmail,
		SellerPhone:    input.SellerPhone,
		BuyerName:      input.BuyerName,
		BuyerAddress:   input.BuyerAddress,
		BuyerEmail:     input.BuyerEmail,
		Logo:           input.Logo,
		ItemsMinor:     input.ItemsMinor,
		ShippingMinor:  input.ShippingMinor,
		DiscountMinor:  input.DiscountMinor,
		TotalMinor:     input.TotalMinor,
		Reason:         input.Reason,
		Paid:           input.Paid,
	}
	for _, line := range input.Lines {
		document.Lines = append(document.Lines, invoicepdf.Line{
			Name:           line.Name,
			Configuration:  line.Configuration,
			Quantity:       line.Quantity,
			UnitPriceMinor: line.UnitPriceMinor,
			TotalMinor:     line.TotalMinor,
		})
	}
	if input.PaymentQR != nil {
		document.PaymentQR = &invoicepdf.PaymentQR{
			PNG:   input.PaymentQR.PNG,
			Label: input.PaymentQR.Label,
		}
	}
	if input.Palette != nil {
		palette, err := input.Palette.palette()
		if err != nil {
			return invoicepdf.Document{}, err
		}
		document.Palette = &palette
	}
	return document, nil
}

func (input paletteInput) palette() (invoicepdf.Palette, error) {
	var palette invoicepdf.Palette
	targets := []struct {
		name  string
		value string
		color *invoicepdf.Color
	}{
		{"primary", input.Primary, &palette.Primary},
		{"surface", input.Surface, &palette.Surface},
		{"accent", input.Accent, &palette.Accent},
		{"text", input.Text, &palette.Text},
		{"muted", input.Muted, &palette.Muted},
		{"rule", input.Rule, &palette.Rule},
		{"row_alt", input.RowAlt, &palette.RowAlt},
		{"white", input.White, &palette.White},
	}
	for _, target := range targets {
		color, err := parseHexColor(target.name, target.value)
		if err != nil {
			return invoicepdf.Palette{}, err
		}
		*target.color = color
	}
	return palette, nil
}

func parseHexColor(name, value string) (invoicepdf.Color, error) {
	if value == "" {
		return invoicepdf.Color{}, nil
	}
	invalid := fmt.Errorf("palette %s: %q is not a #RRGGBB color", name, value)
	if len(value) != 7 || value[0] != '#' {
		return invoicepdf.Color{}, invalid
	}
	parsed, err := strconv.ParseUint(value[1:], 16, 32)
	if err != nil {
		return invoicepdf.Color{}, invalid
	}
	return invoicepdf.Color{
		R: uint8(parsed >> 16),
		G: uint8(parsed >> 8),
		B: uint8(parsed),
	}, nil
}

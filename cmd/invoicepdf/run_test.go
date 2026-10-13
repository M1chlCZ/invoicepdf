package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func samplePNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewGray(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buffer.Bytes()
}

func goodInput(t *testing.T) string {
	t.Helper()
	logo := base64.StdEncoding.EncodeToString(samplePNG(t))
	return fmt.Sprintf(`{
  "number": "2026-0042",
  "kind": "invoice",
  "issued_date": "2026-09-19",
  "supply_date": "2026-09-18",
  "order_number": "OBJ-2026-0042",
  "locale": "cs-CZ",
  "currency": "CZK",
  "seller_name": "Sample Goods s.r.o.",
  "seller_id": "12345678",
  "seller_address": "Dlouhá 12\n110 00 Praha 1",
  "seller_email": "billing@example.com",
  "seller_phone": "+420 123 456 789",
  "buyer_name": "Jan Novák",
  "buyer_address": "Krátká 3\n602 00 Brno",
  "buyer_email": "jan@example.cz",
  "logo_base64": %q,
  "payment_qr": {"png_base64": %q, "label": "QR Platba"},
  "palette": {
    "primary": "#1F2937", "surface": "#F1F5F9", "accent": "#475569",
    "text": "#111827", "muted": "#6B7280", "rule": "#E2E8F0",
    "row_alt": "#F8FAFC", "white": "#FFFFFF"
  },
  "lines": [
    {"name": "Ručně šitá kožená taška", "configuration": "hnědá / 30 cm",
     "quantity": 1, "unit_price_minor": 249000, "total_minor": 249000},
    {"name": "Kožená peněženka", "quantity": 2,
     "unit_price_minor": 45000, "total_minor": 90000}
  ],
  "items_minor": 339000,
  "shipping_minor": 12000,
  "discount_minor": 5000,
  "total_minor": 346000,
  "paid": true
}`, logo, logo)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runCommand(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func decodeSummary(t *testing.T, stdout string) renderSummary {
	t.Helper()
	var summary renderSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode summary %q: %v", stdout, err)
	}
	return summary
}

func TestRunRenderDirectoryToPDFs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inputDir := filepath.Join(root, "in")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(inputDir, "a.json"), goodInput(t))
	writeFile(t, filepath.Join(inputDir, "b.json"), goodInput(t))
	writeFile(t, filepath.Join(inputDir, "ignored.txt"), "not a document")
	outDir := filepath.Join(root, "out")

	code, stdout, stderr := runCommand(t, "render", "--out-dir", outDir, inputDir)
	if code != 0 {
		t.Fatalf("run = %d, want 0; stderr: %s", code, stderr)
	}
	summary := decodeSummary(t, stdout)
	if summary.Version != 1 || summary.Total != 2 || summary.Succeeded != 2 || summary.Failed != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if len(summary.Results) != 2 || !summary.Results[0].OK || summary.Results[0].Error != nil {
		t.Fatalf("results = %+v", summary.Results)
	}
	if got, want := summary.Results[0].Input, filepath.Join(inputDir, "a.json"); got != want {
		t.Fatalf("first input = %q, want %q", got, want)
	}
	if summary.Results[0].ISDOC != "" {
		t.Fatalf("isdoc path without --isdoc = %q", summary.Results[0].ISDOC)
	}
	for _, result := range summary.Results {
		pdf, err := os.ReadFile(result.PDF)
		if err != nil {
			t.Fatalf("read %s: %v", result.PDF, err)
		}
		if !bytes.HasPrefix(pdf, []byte("%PDF")) {
			t.Fatalf("%s is not a PDF", result.PDF)
		}
	}
	if !strings.Contains(stderr, "invoicepdf: [1/2]") || !strings.Contains(stderr, "invoicepdf: [2/2]") {
		t.Fatalf("stderr has no per-file status: %s", stderr)
	}
}

func TestRunRenderISDOCAndDeterministicSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := filepath.Join(root, "invoice.json")
	writeFile(t, input, goodInput(t))
	outDir := filepath.Join(root, "out")

	code, firstStdout, stderr := runCommand(t, "render", "--out-dir", outDir, "--isdoc", input)
	if code != 0 {
		t.Fatalf("run = %d, want 0; stderr: %s", code, stderr)
	}
	summary := decodeSummary(t, firstStdout)
	if summary.Succeeded != 1 || summary.Results[0].ISDOC == "" {
		t.Fatalf("summary = %+v", summary)
	}
	isdoc, err := os.ReadFile(summary.Results[0].ISDOC)
	if err != nil {
		t.Fatalf("read isdoc: %v", err)
	}
	if !bytes.HasPrefix(isdoc, []byte("<?xml")) ||
		!bytes.Contains(isdoc, []byte("<Invoice xmlns=\"http://isdoc.cz/namespace/2013\" version=\"6.0.2\">")) {
		t.Fatalf("isdoc output has no ISDOC header")
	}

	code, secondStdout, stderr := runCommand(t, "render", "--out-dir", outDir, "--isdoc", input)
	if code != 0 {
		t.Fatalf("second run = %d, want 0; stderr: %s", code, stderr)
	}
	if firstStdout != secondStdout {
		t.Fatalf("summary is not deterministic:\n%s\n%s", firstStdout, secondStdout)
	}
}

func TestRunRenderMixedFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inputDir := filepath.Join(root, "in")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(inputDir, "good.json"), goodInput(t))
	writeFile(t, filepath.Join(inputDir, "broken.json"), "not json")
	writeFile(t, filepath.Join(inputDir, "unknown.json"), `{"number": "2026-0001", "unknown_field": true}`)
	writeFile(t, filepath.Join(inputDir, "incomplete.json"),
		`{"issued_date": "2026-09-19", "currency": "CZK", "seller_name": "S", "buyer_name": "B", "lines": [{"name": "x"}]}`)
	outDir := filepath.Join(root, "out")

	code, stdout, stderr := runCommand(t, "render", "--out-dir", outDir, "--isdoc", inputDir)
	if code != 1 {
		t.Fatalf("run = %d, want 1; stderr: %s", code, stderr)
	}
	summary := decodeSummary(t, stdout)
	if summary.Total != 4 || summary.Succeeded != 1 || summary.Failed != 3 {
		t.Fatalf("summary = %+v", summary)
	}
	failed := 0
	for _, result := range summary.Results {
		if result.OK {
			if result.Error != nil || result.PDF == "" || result.ISDOC == "" {
				t.Fatalf("successful result = %+v", result)
			}
			continue
		}
		failed++
		if result.Error == nil || *result.Error == "" || result.PDF != "" || result.ISDOC != "" {
			t.Fatalf("failed result = %+v", result)
		}
	}
	if failed != 3 {
		t.Fatalf("failed result count = %d, want 3", failed)
	}
	if _, err := os.Stat(filepath.Join(outDir, "good.pdf")); err != nil {
		t.Fatalf("good document has no PDF: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "good.isdoc")); err != nil {
		t.Fatalf("good document has no ISDOC: %v", err)
	}
	for _, name := range []string{"broken.pdf", "unknown.pdf", "incomplete.pdf", "incomplete.isdoc"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err == nil {
			t.Fatalf("failed document wrote %s", name)
		}
	}
	if !strings.Contains(stderr, "failed:") {
		t.Fatalf("stderr has no failure status: %s", stderr)
	}
}

func TestRunRenderExplicitInputOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first := filepath.Join(root, "b.json")
	second := filepath.Join(root, "a.json")
	writeFile(t, first, goodInput(t))
	writeFile(t, second, goodInput(t))

	code, stdout, stderr := runCommand(t, "render", "--out-dir", filepath.Join(root, "out"), first, second)
	if code != 0 {
		t.Fatalf("run = %d, want 0; stderr: %s", code, stderr)
	}
	summary := decodeSummary(t, stdout)
	if summary.Results[0].Input != first || summary.Results[1].Input != second {
		t.Fatalf("explicit input order was not preserved: %+v", summary.Results)
	}
}

func TestRunRenderRejectsInvalidPalette(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "bad.json")
	writeFile(t, path, `{"palette": {"primary": "red"}}`)

	code, stdout, stderr := runCommand(t, "render", "--out-dir", filepath.Join(root, "out"), path)
	if code != 1 {
		t.Fatalf("run = %d, want 1; stderr: %s", code, stderr)
	}
	summary := decodeSummary(t, stdout)
	if summary.Failed != 1 || summary.Results[0].Error == nil ||
		!strings.Contains(*summary.Results[0].Error, "palette primary") {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestRunUsageErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	output := filepath.Join(root, "out")
	cases := [][]string{
		{},
		{"convert"},
		{"render"},
		{"render", "--out-dir", output},
		{"render", filepath.Join(root, "a.json")},
		{"render", "--unknown", "--out-dir", output, filepath.Join(root, "a.json")},
		{"render", "--out-dir", output, filepath.Join(root, "missing.json")},
	}
	for _, args := range cases {
		code, stdout, stderr := runCommand(t, args...)
		if code != 2 {
			t.Fatalf("run(%v) = %d, want 2; stderr: %s", args, code, stderr)
		}
		if stdout != "" {
			t.Fatalf("run(%v) wrote stdout %q", args, stdout)
		}
		if stderr == "" {
			t.Fatalf("run(%v) wrote no usage message", args)
		}
	}
}

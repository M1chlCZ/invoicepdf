package invoicepdf_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/m1chlcz/invoicepdf"
)

func TestLookupLabelsCzech(t *testing.T) {
	t.Parallel()
	labels, err := invoicepdf.LookupLabels("cs-CZ")
	if err != nil {
		t.Fatalf("LookupLabels(cs-CZ): %v", err)
	}
	expected := invoicepdf.Labels{
		InvoiceTitle:    "Faktura",
		CreditNoteTitle: "Opravný doklad",
		OriginalNumber:  "Původní doklad",
		IssuedDate:      "Datum vystavení",
		SupplyDate:      "Datum dodání",
		OrderNumber:     "Číslo objednávky",
		Seller:          "Prodávající",
		Buyer:           "Odběratel",
		SellerID:        "IČO",
		Description:     "Popis",
		Quantity:        "Množství",
		UnitPrice:       "Cena za jednotku",
		LineTotal:       "Celkem",
		ItemsTotal:      "Položky celkem",
		Shipping:        "Doprava",
		Discount:        "Sleva",
		TotalDue:        "Celkem k úhradě",
		TotalSettled:    "Celkem",
		PaidNotice:      "Neplaťte — již uhrazeno.",
		Unpaid:          "Nezaplaceno",
		Reason:          "Důvod opravy",
		NotVATPayer:     "Prodávající není plátcem DPH.",
		Continued:       "pokračování",
		Page:            "Strana",
		EmptyItems:      "Bez položek",
	}
	if labels != expected {
		t.Fatalf("LookupLabels(cs-CZ) = %+v, want %+v", labels, expected)
	}
}

func TestLookupLabelsEnglish(t *testing.T) {
	t.Parallel()
	labels, err := invoicepdf.LookupLabels("en-GB")
	if err != nil {
		t.Fatalf("LookupLabels(en-GB): %v", err)
	}
	expected := invoicepdf.Labels{
		InvoiceTitle:    "Invoice",
		CreditNoteTitle: "Credit note",
		OriginalNumber:  "Original document",
		IssuedDate:      "Issue date",
		SupplyDate:      "Supply date",
		OrderNumber:     "Order number",
		Seller:          "Seller",
		Buyer:           "Buyer",
		SellerID:        "Company ID",
		Description:     "Description",
		Quantity:        "Qty",
		UnitPrice:       "Unit price",
		LineTotal:       "Total",
		ItemsTotal:      "Items total",
		Shipping:        "Shipping",
		Discount:        "Discount",
		TotalDue:        "Total due",
		TotalSettled:    "Total",
		PaidNotice:      "Already paid — do not pay again.",
		Unpaid:          "Unpaid",
		Reason:          "Reason for correction",
		NotVATPayer:     "The seller is not a VAT payer.",
		Continued:       "continued",
		Page:            "Page",
		EmptyItems:      "No items",
	}
	if labels != expected {
		t.Fatalf("LookupLabels(en-GB) = %+v, want %+v", labels, expected)
	}

	empty, err := invoicepdf.LookupLabels("")
	if err != nil {
		t.Fatalf("LookupLabels(empty): %v", err)
	}
	if empty != expected {
		t.Fatalf("LookupLabels(empty) must match the English fallback of Render")
	}

	padded, err := invoicepdf.LookupLabels("  CS-cz ")
	if err != nil {
		t.Fatalf("LookupLabels(padded): %v", err)
	}
	if padded.InvoiceTitle != "Faktura" {
		t.Fatalf("LookupLabels(padded).InvoiceTitle = %q, want Czech", padded.InvoiceTitle)
	}
}

func TestLookupLabelsRejectsUnsupportedLocale(t *testing.T) {
	t.Parallel()
	labels, err := invoicepdf.LookupLabels("de-DE")
	if err == nil {
		t.Fatalf("LookupLabels(de-DE) succeeded with %+v, want error", labels)
	}
	if !errors.Is(err, invoicepdf.ErrUnsupportedLocale) {
		t.Fatalf("LookupLabels(de-DE) error = %v, want ErrUnsupportedLocale", err)
	}
	if !strings.Contains(err.Error(), "de-DE") {
		t.Fatalf("LookupLabels(de-DE) error = %v, want the locale in the message", err)
	}
}

// Package invoicepdf renders invoices and credit notes as PDF bytes.
//
// A Document supplies snapshot data, including minor-unit amounts and an
// optional logo and palette. Render returns complete PDF bytes and starts
// new pages with a repeated table header when content overflows.
package invoicepdf

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Document is the immutable snapshot rendered into a single invoice or credit-note PDF.
type Document struct {
	Number, Kind, OriginalNumber, IssuedDate, SupplyDate, OrderNumber, Locale, Currency                string
	SellerName, SellerID, SellerAddress, SellerEmail, SellerPhone, BuyerName, BuyerAddress, BuyerEmail string
	Logo                                                                                               []byte
	PaymentQR                                                                                          *PaymentQR
	Palette                                                                                            *Palette
	Lines                                                                                              []Line
	ItemsMinor, ShippingMinor, DiscountMinor, TotalMinor                                               int64
	Reason                                                                                             string
	Paid                                                                                               bool
}

// PaymentQR holds an optional payment QR image. The renderer draws the image
// next to the totals and the payment notice. PNG holds encoded PNG bytes; the
// qrplatba tool generates them. The renderer prints Label centered under the
// image; an empty Label draws no caption.
type PaymentQR struct {
	PNG   []byte
	Label string
}

// Line is one rendered line item. Minor amounts are in the currency minor
// unit; a credit note uses negative values.
type Line struct {
	Name, Configuration                  string
	Quantity, UnitPriceMinor, TotalMinor int64
}

// Document kinds accepted by Render.
const (
	KindInvoice    = "invoice"
	KindCreditNote = "credit_note"
)

const (
	fontRegular = "invoicepdf-regular"
	fontBold    = "invoicepdf-bold"

	lineSpacing     = 1.32
	pageWidth       = 595.0
	pageHeight      = 842.0
	margin          = 46.0
	footerInset     = 54.0
	logoRatio       = 3.0
	fractionDigits  = 2
	minorMinLength  = fractionDigits + 1
	groupSize       = 3
	dateLayoutISO   = "2006-01-02"
	dateLayoutCS    = "02.01.2006"
	dateLayoutEN    = "02 Jan 2006"
	pageCountFormat = "%s %d / %d"
	labelTotalCS    = "Celkem"
	labelTotalEN    = "Total"
	labelShippingCS = "Doprava"
	labelShippingEN = "Shipping"

	headerHeight    = 150.0
	headerGap       = 26.0
	headerLogoY     = 32.0
	headerLogoWidth = 168.0
	titleY          = 34.0
	numberY         = 74.0
	titleSize       = 26.0
	numberSize      = 12.0
	headerRuleWidth = 3.0

	contHeaderHeight = 64.0
	contHeaderGap    = 24.0
	contLogoY        = 14.0
	contLogoWidth    = 108.0
	contCaptionY     = 26.0
	contCaptionSize  = 9.5
	contRuleWidth    = 2.4

	partyHeadingSize       = 8.5
	partyHeadingGap        = 14.0
	partyNameSize          = 11.0
	partyNameLineHeight    = 14.0
	partyAddressSize       = 9.5
	partyAddressLineHeight = 12.5
	partyMetaSize          = 9.0
	partyMetaLineHeight    = 12.0
	partySectionGap        = 16.0
	partyMinSpace          = 30.0

	metaHeight     = 48.0
	metaGap        = 8.0
	metaLabelY     = 11.0
	metaValueY     = 24.0
	metaLabelSize  = 7.5
	metaValueSize  = 9.5
	metaXInset     = 14.0
	metaWidthInset = 20.0

	tableGap          = 12.0
	tableQtyWidth     = 44.0
	tableUnitWidth    = 92.0
	tableTotalWidth   = 92.0
	tableHeaderHeight = 24.0
	tableHeaderTextY  = 7.0
	tableHeaderSize   = 8.2
	tableDescInset    = 8.0
	tableNumInset     = 8.0
	tableGapTotal     = 3 * tableGap

	rowPad              = 7.0
	rowVerticalPad      = 2 * rowPad
	rowNameSize         = 9.0
	rowNameLineHeight   = 11.6
	rowConfigSize       = 8.0
	rowConfigLineHeight = 10.2
	rowMinHeight        = 26.0
	rowSpacer           = 3.0
	rowRuleWidth        = 0.6
	itemsGap            = 10.0
	emptyRowHeight      = 26.0

	totalsAdvance     = 16.0
	totalsBoxWidth    = 252.0
	totalsRowHeight   = 16.0
	totalsValueWidth  = 118.0
	totalsRowSize     = 9.5
	totalsRuleOffset  = 9.0
	totalsRuleWidth   = 1.2
	totalsBoxHeight   = 32.0
	totalsLabelX      = 14.0
	totalsLabelY      = 10.0
	totalsLabelSize   = 10.0
	totalsLabelGap    = 20.0
	totalsLabelWidth  = totalsBoxWidth - totalsValueWidth - totalsLabelGap
	totalsAmountY     = 8.0
	totalsAmountSize  = 13.0
	totalsAmountInset = 14.0
	totalsGap         = 14.0
	totalsMinHeight   = 46.0

	paymentQRSize            = 96.0
	paymentQRLabelGap        = 6.0
	paymentQRLabelSize       = 8.0
	paymentQRLabelLineHeight = 10.5

	noticeHeight      = 40.0
	noticeGap         = 12.0
	noticeRuleWidth   = 1.2
	noticeAccentWidth = 6.0
	noticeTextX       = 18.0
	noticeTextY       = 12.0
	noticeTextSize    = 12.0
	noticeTextInset   = 28.0

	reasonSpace       = 46.0
	reasonHeadingSize = 8.5
	reasonHeadingGap  = 15.0
	reasonSize        = 9.0
	reasonLineHeight  = 12.0
	reasonGap         = 6.0
	taxNoteSpace      = 26.0
	taxNoteSize       = 8.5
	taxNoteY          = 6.0

	footerRuleOffset = 34.0
	footerRuleWidth  = 0.7
	footerTextY      = 7.0
	footerSize       = 7.5
)

// Color is an RGB color value.
type Color struct {
	R, G, B uint8
}

// Palette replaces the built-in document colors.
type Palette struct {
	Primary Color
	Surface Color
	Accent  Color
	Text    Color
	Muted   Color
	Rule    Color
	RowAlt  Color
	White   Color
}

func defaultPalette() Palette {
	return Palette{
		Primary: Color{0x1F, 0x29, 0x37},
		Surface: Color{0xF1, 0xF5, 0xF9},
		Accent:  Color{0x47, 0x55, 0x69},
		Text:    Color{0x11, 0x18, 0x27},
		Muted:   Color{0x6B, 0x72, 0x80},
		Rule:    Color{0xE2, 0xE8, 0xF0},
		RowAlt:  Color{0xF8, 0xFA, 0xFC},
		White:   Color{0xFF, 0xFF, 0xFF},
	}
}

type labels struct {
	invoiceTitle    string
	creditNoteTitle string
	originalNumber  string
	issuedDate      string
	supplyDate      string
	orderNumber     string
	seller          string
	buyer           string
	sellerID        string
	description     string
	quantity        string
	unitPrice       string
	lineTotal       string
	itemsTotal      string
	shipping        string
	discount        string
	totalDue        string
	totalSettled    string
	paidNotice      string
	unpaid          string
	reason          string
	notVATPayer     string
	continued       string
	page            string
	emptyItems      string
}

func labelsFor(czech bool) labels {
	if czech {
		return labels{ //nolint:gosec // Localized UI label text, not hardcoded credentials.
			invoiceTitle:    "Faktura",
			creditNoteTitle: "Opravný doklad",
			originalNumber:  "Původní doklad",
			issuedDate:      "Datum vystavení",
			supplyDate:      "Datum dodání",
			orderNumber:     "Číslo objednávky",
			seller:          "Prodávající",
			buyer:           "Odběratel",
			sellerID:        "IČO",
			description:     "Popis",
			quantity:        "Množství",
			unitPrice:       "Cena za jednotku",
			lineTotal:       labelTotalCS,
			itemsTotal:      "Položky celkem",
			shipping:        labelShippingCS,
			discount:        "Sleva",
			totalDue:        "Celkem k úhradě",
			totalSettled:    labelTotalCS,
			paidNotice:      "Neplaťte — již uhrazeno.",
			unpaid:          "Nezaplaceno",
			reason:          "Důvod opravy",
			notVATPayer:     "Prodávající není plátcem DPH.",
			continued:       "pokračování",
			page:            "Strana",
			emptyItems:      "Bez položek",
		}
	}
	return labels{
		invoiceTitle:    "Invoice",
		creditNoteTitle: "Credit note",
		originalNumber:  "Original document",
		issuedDate:      "Issue date",
		supplyDate:      "Supply date",
		orderNumber:     "Order number",
		seller:          "Seller",
		buyer:           "Buyer",
		sellerID:        "Company ID",
		description:     "Description",
		quantity:        "Qty",
		unitPrice:       "Unit price",
		lineTotal:       labelTotalEN,
		itemsTotal:      "Items total",
		shipping:        labelShippingEN,
		discount:        "Discount",
		totalDue:        "Total due",
		totalSettled:    labelTotalEN,
		paidNotice:      "Already paid — do not pay again.",
		unpaid:          "Unpaid",
		reason:          "Reason for correction",
		notVATPayer:     "The seller is not a VAT payer.",
		continued:       "continued",
		page:            "Page",
		emptyItems:      "No items",
	}
}

type tableLayout struct {
	descX, qtyX, unitX, totalX float64
	descW, qtyW, unitW, totalW float64
}

type totalRow struct {
	label string
	value int64
}

type paymentQRLayout struct {
	png    []byte
	lines  []string
	size   float64
	height float64
}

type rowFragment struct {
	text   string
	size   float64
	color  Color
	height float64
}

type renderer struct {
	pdf     gopdf.GoPdf
	doc     Document
	labels  labels
	palette Palette
	czech   bool

	width  float64
	height float64
	left   float64
	right  float64
	bottom float64
	y      float64
}

// Render returns a complete PDF as bytes and never changes the document.
func Render(document Document) ([]byte, error) {
	renderer := newRenderer(document)
	if err := renderer.render(); err != nil {
		return nil, err
	}
	return renderer.pdf.GetBytesPdfReturnErr()
}

func newRenderer(document Document) *renderer {
	czech := isCzech(document.Locale)
	palette := defaultPalette()
	if document.Palette != nil {
		palette = *document.Palette
	}
	return &renderer{
		doc:     document,
		labels:  labelsFor(czech),
		palette: palette,
		czech:   czech,
		width:   pageWidth,
		height:  pageHeight,
		left:    margin,
		right:   pageWidth - margin,
		bottom:  pageHeight - footerInset,
	}
}

func (r *renderer) render() error {
	if err := r.validatePaymentQR(); err != nil {
		return err
	}
	r.pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: pageWidth, H: pageHeight},
		Unit:     gopdf.UnitPT,
	})
	if err := r.pdf.AddTTFFontData(fontRegular, goregular.TTF); err != nil {
		return err
	}
	if err := r.pdf.AddTTFFontData(fontBold, gobold.TTF); err != nil {
		return err
	}
	r.pdf.SetInfo(gopdf.PdfInfo{
		Title:    r.documentTitle() + " " + strings.TrimSpace(r.doc.Number),
		Author:   strings.TrimSpace(r.doc.SellerName),
		Creator:  "invoicepdf",
		Producer: "invoicepdf",
	})

	r.pdf.AddPage()
	if err := r.drawDocumentHeader(); err != nil {
		return err
	}
	if err := r.drawParties(); err != nil {
		return err
	}
	if err := r.drawMetaBand(); err != nil {
		return err
	}
	if err := r.drawItemsTable(); err != nil {
		return err
	}
	if err := r.drawTotals(); err != nil {
		return err
	}
	if err := r.drawPaymentNotice(); err != nil {
		return err
	}
	if err := r.drawNotes(); err != nil {
		return err
	}
	return r.drawFooters()
}

func (r *renderer) validatePaymentQR() error {
	if r.doc.PaymentQR == nil {
		return nil
	}
	if len(r.doc.PaymentQR.PNG) == 0 {
		return errors.New("invoicepdf: payment qr: png data is empty")
	}
	if _, err := png.Decode(bytes.NewReader(r.doc.PaymentQR.PNG)); err != nil {
		return fmt.Errorf("invoicepdf: payment qr: invalid png data: %w", err)
	}
	return nil
}

func (r *renderer) contentWidth() float64 {
	return r.right - r.left
}

func (r *renderer) isCreditNote() bool {
	return strings.EqualFold(strings.TrimSpace(r.doc.Kind), KindCreditNote)
}

func (r *renderer) documentTitle() string {
	if r.isCreditNote() {
		return r.labels.creditNoteTitle
	}
	return r.labels.invoiceTitle
}

func (r *renderer) setFillColor(color Color) {
	r.pdf.SetFillColor(color.R, color.G, color.B)
}

func (r *renderer) setStrokeColor(color Color) {
	r.pdf.SetStrokeColor(color.R, color.G, color.B)
}

func (r *renderer) setTextColor(color Color) {
	r.pdf.SetTextColor(color.R, color.G, color.B)
}

func (r *renderer) setFont(bold bool, size float64) error {
	family := fontRegular
	if bold {
		family = fontBold
	}
	return r.pdf.SetFont(family, "", size)
}

func (r *renderer) drawText(
	text string,
	x, y, width float64,
	bold bool,
	size float64,
	color Color,
	align int,
) error {
	if err := r.setFont(bold, size); err != nil {
		return err
	}
	r.setTextColor(color)
	r.pdf.SetXY(x, y)
	return r.pdf.CellWithOption(
		&gopdf.Rect{W: width, H: size * lineSpacing},
		text,
		gopdf.CellOption{Align: align | gopdf.Top},
	)
}

func (r *renderer) drawLogo(x, y, width float64) error {
	if len(r.doc.Logo) == 0 {
		return nil
	}
	holder, err := gopdf.ImageHolderByBytes(r.doc.Logo)
	if err != nil {
		return err
	}
	return r.pdf.ImageByHolder(holder, x, y, &gopdf.Rect{W: width, H: width / logoRatio})
}

func (r *renderer) wrapText(
	text string,
	width float64,
	bold bool,
	size float64,
) ([]string, error) {
	if err := r.setFont(bold, size); err != nil {
		return nil, err
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var lines []string
	for paragraph := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(paragraph)
		if trimmed == "" {
			lines = append(lines, "")
			continue
		}
		wrapped, err := r.wrapParagraph(trimmed, width)
		if err != nil {
			return nil, err
		}
		lines = append(lines, wrapped...)
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines, nil
}

func (r *renderer) wrapParagraph(paragraph string, width float64) ([]string, error) {
	var lines []string
	current := ""
	for word := range strings.FieldsSeq(paragraph) {
		if current == "" {
			current = word
		} else {
			fits, fitErr := r.textFits(current+" "+word, width)
			if fitErr != nil {
				return nil, fitErr
			}
			if fits {
				current += " " + word
			} else {
				lines = append(lines, current)
				current = word
			}
		}
		broken, remainder, breakErr := r.breakOversized(current, width)
		if breakErr != nil {
			return nil, breakErr
		}
		lines = append(lines, broken...)
		current = remainder
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines, nil
}

func (r *renderer) breakOversized(text string, width float64) ([]string, string, error) {
	var lines []string
	for {
		fits, fitErr := r.textFits(text, width)
		if fitErr != nil {
			return nil, "", fitErr
		}
		if fits || utf8.RuneCountInString(text) <= 1 {
			return lines, text, nil
		}
		runes := []rune(text)
		cut := len(runes) - 1
		for cut > 1 {
			prefixFits, prefixErr := r.textFits(string(runes[:cut]), width)
			if prefixErr != nil {
				return nil, "", prefixErr
			}
			if prefixFits {
				break
			}
			cut--
		}
		lines = append(lines, string(runes[:cut]))
		text = string(runes[cut:])
	}
}

func (r *renderer) textFits(text string, width float64) (bool, error) {
	measured, err := r.pdf.MeasureTextWidth(text)
	if err != nil {
		return false, err
	}
	return measured <= width, nil
}

func (r *renderer) drawDocumentHeader() error {
	r.setFillColor(r.palette.Surface)
	r.pdf.RectFromUpperLeftWithStyle(0, 0, r.width, headerHeight, "F")

	if err := r.drawLogo(r.left, headerLogoY, headerLogoWidth); err != nil {
		return err
	}
	if err := r.drawText(
		r.documentTitle(),
		r.left,
		titleY,
		r.contentWidth(),
		true,
		titleSize,
		r.palette.Primary,
		gopdf.Right,
	); err != nil {
		return err
	}
	if err := r.drawText(
		strings.TrimSpace(r.doc.Number),
		r.left,
		numberY,
		r.contentWidth(),
		false,
		numberSize,
		r.palette.Text,
		gopdf.Right,
	); err != nil {
		return err
	}

	r.setStrokeColor(r.palette.Accent)
	r.pdf.SetLineWidth(headerRuleWidth)
	r.pdf.Line(0, headerHeight, r.width, headerHeight)

	r.y = headerHeight + headerGap
	return nil
}

func (r *renderer) drawContinuationHeader() error {
	r.setFillColor(r.palette.Surface)
	r.pdf.RectFromUpperLeftWithStyle(0, 0, r.width, contHeaderHeight, "F")

	if err := r.drawLogo(r.left, contLogoY, contLogoWidth); err != nil {
		return err
	}
	caption := r.documentTitle()
	if number := strings.TrimSpace(r.doc.Number); number != "" {
		caption += " " + number
	}
	caption += " · " + r.labels.continued
	if err := r.drawText(
		caption,
		r.left,
		contCaptionY,
		r.contentWidth(),
		false,
		contCaptionSize,
		r.palette.Muted,
		gopdf.Right,
	); err != nil {
		return err
	}

	r.setStrokeColor(r.palette.Accent)
	r.pdf.SetLineWidth(contRuleWidth)
	r.pdf.Line(0, contHeaderHeight, r.width, contHeaderHeight)

	r.y = contHeaderHeight + contHeaderGap
	return nil
}

func (r *renderer) ensureSpace(height float64) error {
	if r.y+height <= r.bottom {
		return nil
	}
	r.pdf.AddPage()
	return r.drawContinuationHeader()
}

func (r *renderer) drawWrappedValue(
	value string,
	bold bool,
	size, lineHeight float64,
	color Color,
) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	lines, err := r.wrapText(value, r.contentWidth(), bold, size)
	if err != nil {
		return err
	}
	return r.drawWrappedLines(lines, bold, size, lineHeight, color)
}

func (r *renderer) drawWrappedLines(
	lines []string,
	bold bool,
	size, lineHeight float64,
	color Color,
) error {
	for _, line := range lines {
		if err := r.ensureSpace(lineHeight); err != nil {
			return err
		}
		if err := r.drawText(
			line,
			r.left,
			r.y,
			r.contentWidth(),
			bold,
			size,
			color,
			gopdf.Left,
		); err != nil {
			return err
		}
		r.y += lineHeight
	}
	return nil
}

func (r *renderer) drawParties() error {
	if err := r.drawPartySection(
		r.labels.seller,
		r.doc.SellerName,
		r.doc.SellerAddress,
		r.doc.SellerEmail,
		r.doc.SellerPhone,
		r.doc.SellerID,
	); err != nil {
		return err
	}
	r.y += partySectionGap
	return r.drawPartySection(
		r.labels.buyer,
		r.doc.BuyerName,
		r.doc.BuyerAddress,
		r.doc.BuyerEmail,
		"",
		"",
	)
}

func (r *renderer) drawPartySection(heading, name, address, email, phone, id string) error {
	if err := r.ensureSpace(partyMinSpace); err != nil {
		return err
	}
	if err := r.drawText(
		heading,
		r.left,
		r.y,
		r.contentWidth(),
		true,
		partyHeadingSize,
		r.palette.Accent,
		gopdf.Left,
	); err != nil {
		return err
	}
	r.y += partyHeadingGap
	if err := r.drawWrappedValue(
		name, true, partyNameSize, partyNameLineHeight, r.palette.Text,
	); err != nil {
		return err
	}
	if err := r.drawWrappedValue(
		address, false, partyAddressSize, partyAddressLineHeight, r.palette.Text,
	); err != nil {
		return err
	}
	if trimmed := strings.TrimSpace(id); trimmed != "" {
		if err := r.drawWrappedValue(
			r.labels.sellerID+": "+trimmed,
			false,
			partyMetaSize,
			partyMetaLineHeight,
			r.palette.Muted,
		); err != nil {
			return err
		}
	}
	if err := r.drawWrappedValue(
		email, false, partyMetaSize, partyMetaLineHeight, r.palette.Muted,
	); err != nil {
		return err
	}
	return r.drawWrappedValue(phone, false, partyMetaSize, partyMetaLineHeight, r.palette.Muted)
}

func (r *renderer) drawMetaBand() error {
	var items []metaItem
	if value := formatDate(r.doc.IssuedDate, r.czech); value != "" {
		items = append(items, metaItem{r.labels.issuedDate, value})
	}
	if value := formatDate(r.doc.SupplyDate, r.czech); value != "" {
		items = append(items, metaItem{r.labels.supplyDate, value})
	}
	if order := strings.TrimSpace(r.doc.OrderNumber); order != "" {
		items = append(items, metaItem{r.labels.orderNumber, order})
	}
	if r.isCreditNote() {
		if original := strings.TrimSpace(r.doc.OriginalNumber); original != "" {
			items = append(items, metaItem{r.labels.originalNumber, original})
		}
	}
	if len(items) == 0 {
		return nil
	}
	if err := r.ensureSpace(metaHeight + metaGap); err != nil {
		return err
	}

	r.setFillColor(r.palette.Surface)
	r.pdf.RectFromUpperLeftWithStyle(r.left, r.y, r.contentWidth(), metaHeight, "F")
	columnWidth := r.contentWidth() / float64(len(items))
	for index, item := range items {
		x := r.left + metaXInset + float64(index)*columnWidth
		width := columnWidth - metaWidthInset
		if err := r.drawText(
			item.label,
			x,
			r.y+metaLabelY,
			width,
			false,
			metaLabelSize,
			r.palette.Muted,
			gopdf.Left,
		); err != nil {
			return err
		}
		if err := r.drawText(
			item.value,
			x,
			r.y+metaValueY,
			width,
			true,
			metaValueSize,
			r.palette.Text,
			gopdf.Left,
		); err != nil {
			return err
		}
	}
	r.y += metaHeight + metaGap
	return nil
}

type metaItem struct {
	label string
	value string
}

func (r *renderer) tableLayout() tableLayout {
	descWidth := r.contentWidth() -
		tableQtyWidth -
		tableUnitWidth -
		tableTotalWidth -
		tableGapTotal
	return tableLayout{
		descX:  r.left,
		descW:  descWidth,
		qtyX:   r.left + descWidth + tableGap,
		qtyW:   tableQtyWidth,
		unitX:  r.left + descWidth + tableGap + tableQtyWidth + tableGap,
		unitW:  tableUnitWidth,
		totalX: r.left + descWidth + tableGap + tableQtyWidth + tableGap + tableUnitWidth + tableGap,
		totalW: tableTotalWidth,
	}
}

func (r *renderer) tablePageCapacity() float64 {
	return r.bottom -
		(contHeaderHeight + contHeaderGap + tableHeaderHeight)
}

func (r *renderer) moneyHeader(label string) string {
	currency := currencyLabel(r.doc.Currency, r.czech)
	if currency == "" {
		return label
	}
	return label + " (" + currency + ")"
}

func (r *renderer) drawTableHeader(layout tableLayout) error {
	r.setFillColor(r.palette.Primary)
	r.pdf.RectFromUpperLeftWithStyle(r.left, r.y, r.contentWidth(), tableHeaderHeight, "F")

	top := r.y + tableHeaderTextY
	headers := []tableHeader{
		{
			text:  r.labels.description,
			x:     layout.descX + tableDescInset,
			width: layout.descW - 2*tableDescInset,
			align: gopdf.Left,
		},
		{text: r.labels.quantity, x: layout.qtyX, width: layout.qtyW - tableNumInset, align: gopdf.Right},
		{
			text:  r.moneyHeader(r.labels.unitPrice),
			x:     layout.unitX,
			width: layout.unitW - tableNumInset,
			align: gopdf.Right,
		},
		{
			text:  r.moneyHeader(r.labels.lineTotal),
			x:     layout.totalX,
			width: layout.totalW - tableNumInset,
			align: gopdf.Right,
		},
	}
	for _, header := range headers {
		if err := r.drawText(
			header.text,
			header.x,
			top,
			header.width,
			true,
			tableHeaderSize,
			r.palette.Surface,
			header.align,
		); err != nil {
			return err
		}
	}
	r.y += tableHeaderHeight
	return nil
}

type tableHeader struct {
	text  string
	x     float64
	width float64
	align int
}

func (r *renderer) drawItemsTable() error {
	layout := r.tableLayout()
	if err := r.drawTableHeader(layout); err != nil {
		return err
	}
	if len(r.doc.Lines) == 0 {
		if err := r.drawEmptyItemsRow(layout); err != nil {
			return err
		}
	} else {
		for index, line := range r.doc.Lines {
			if err := r.drawItemRow(layout, index, line); err != nil {
				return err
			}
		}
	}
	r.y += itemsGap
	return nil
}

func (r *renderer) drawEmptyItemsRow(layout tableLayout) error {
	if r.y+emptyRowHeight > r.bottom {
		if err := r.startContinuationPage(layout); err != nil {
			return err
		}
	}
	if err := r.drawText(
		r.labels.emptyItems,
		layout.descX+tableDescInset,
		r.y+tableDescInset,
		layout.descW-2*tableDescInset,
		false,
		rowNameSize,
		r.palette.Muted,
		gopdf.Left,
	); err != nil {
		return err
	}
	r.y += emptyRowHeight
	return nil
}

func (r *renderer) drawItemRow(layout tableLayout, index int, line Line) error {
	descriptionWidth := layout.descW - rowVerticalPad
	fragments, err := r.itemRowFragments(line, descriptionWidth)
	if err != nil {
		return err
	}
	rowHeight := r.itemRowHeight(fragments)
	if rowHeight > r.tablePageCapacity() {
		return r.drawFragmentedItemRow(layout, line, fragments)
	}
	if r.y+rowHeight > r.bottom {
		if err = r.startContinuationPage(layout); err != nil {
			return err
		}
	}
	if index%2 == 1 {
		r.setFillColor(r.palette.RowAlt)
		r.pdf.RectFromUpperLeftWithStyle(r.left, r.y, r.contentWidth(), rowHeight, "F")
	}
	if err = r.drawRowLines(layout, fragments, descriptionWidth, r.y+rowPad); err != nil {
		return err
	}
	if err = r.drawItemNumbers(layout, line, r.y+rowPad); err != nil {
		return err
	}
	r.drawItemRule(r.y + rowHeight)
	r.y += rowHeight
	return nil
}

func (r *renderer) itemRowFragments(
	line Line,
	descriptionWidth float64,
) ([]rowFragment, error) {
	nameLines, err := r.wrapText(line.Name, descriptionWidth, false, rowNameSize)
	if err != nil {
		return nil, err
	}
	var configLines []string
	if configuration := strings.TrimSpace(line.Configuration); configuration != "" {
		configLines, err = r.wrapText(configuration, descriptionWidth, false, rowConfigSize)
		if err != nil {
			return nil, err
		}
	}
	fragments := make([]rowFragment, 0, len(nameLines)+len(configLines)+1)
	for _, nameLine := range nameLines {
		fragments = append(fragments, rowFragment{
			text:   nameLine,
			size:   rowNameSize,
			color:  r.palette.Text,
			height: rowNameLineHeight,
		})
	}
	if len(configLines) > 0 {
		fragments = append(fragments, rowFragment{height: rowSpacer})
		for _, configLine := range configLines {
			fragments = append(fragments, rowFragment{
				text:   configLine,
				size:   rowConfigSize,
				color:  r.palette.Muted,
				height: rowConfigLineHeight,
			})
		}
	}
	return fragments, nil
}

func (r *renderer) itemRowHeight(fragments []rowFragment) float64 {
	height := rowVerticalPad
	for _, fragment := range fragments {
		height += fragment.height
	}
	if height < rowMinHeight {
		return rowMinHeight
	}
	return height
}

func (r *renderer) drawRowLines(
	layout tableLayout,
	fragments []rowFragment,
	width, startY float64,
) error {
	textY := startY
	for _, fragment := range fragments {
		if fragment.size == 0 {
			textY += fragment.height
			continue
		}
		if err := r.drawText(
			fragment.text,
			layout.descX+rowPad,
			textY,
			width,
			false,
			fragment.size,
			fragment.color,
			gopdf.Left,
		); err != nil {
			return err
		}
		textY += fragment.height
	}
	return nil
}

func (r *renderer) drawFragmentedItemRow(
	layout tableLayout,
	line Line,
	fragments []rowFragment,
) error {
	numbersDrawn := false
	for _, fragment := range fragments {
		drawn, err := r.drawItemFragment(layout, line, fragment, numbersDrawn)
		if err != nil {
			return err
		}
		numbersDrawn = drawn
	}
	r.drawItemRule(r.y)
	return nil
}

func (r *renderer) drawItemFragment(
	layout tableLayout,
	line Line,
	fragment rowFragment,
	numbersDrawn bool,
) (bool, error) {
	if fragment.size == 0 {
		if r.y+fragment.height <= r.bottom {
			r.y += fragment.height
		}
		return numbersDrawn, nil
	}
	if err := r.ensureFragmentSpace(layout, fragment.height, numbersDrawn); err != nil {
		return numbersDrawn, err
	}
	return r.drawFragmentContent(layout, line, fragment, numbersDrawn)
}

func (r *renderer) ensureFragmentSpace(
	layout tableLayout,
	height float64,
	numbersDrawn bool,
) error {
	if r.y+height <= r.bottom {
		return nil
	}
	if numbersDrawn {
		r.drawItemRule(r.y)
	}
	return r.startContinuationPage(layout)
}

func (r *renderer) drawFragmentContent(
	layout tableLayout,
	line Line,
	fragment rowFragment,
	numbersDrawn bool,
) (bool, error) {
	if !numbersDrawn {
		if err := r.drawItemNumbers(layout, line, r.y+rowPad); err != nil {
			return numbersDrawn, err
		}
		numbersDrawn = true
	}
	if err := r.drawText(
		fragment.text,
		layout.descX+rowPad,
		r.y,
		layout.descW-rowVerticalPad,
		false,
		fragment.size,
		fragment.color,
		gopdf.Left,
	); err != nil {
		return numbersDrawn, err
	}
	r.y += fragment.height
	return numbersDrawn, nil
}

func (r *renderer) drawItemNumbers(
	layout tableLayout,
	line Line,
	y float64,
) error {
	if err := r.drawText(
		strconv.FormatInt(line.Quantity, 10),
		layout.qtyX,
		y,
		layout.qtyW-rowPad,
		false,
		rowNameSize,
		r.palette.Text,
		gopdf.Right,
	); err != nil {
		return err
	}
	if err := r.drawText(
		formatMinorAmount(line.UnitPriceMinor, r.czech),
		layout.unitX,
		y,
		layout.unitW-rowPad,
		false,
		rowNameSize,
		r.palette.Text,
		gopdf.Right,
	); err != nil {
		return err
	}
	return r.drawText(
		formatMinorAmount(line.TotalMinor, r.czech),
		layout.totalX,
		y,
		layout.totalW-rowPad,
		false,
		rowNameSize,
		r.palette.Text,
		gopdf.Right,
	)
}

func (r *renderer) drawItemRule(y float64) {
	r.setStrokeColor(r.palette.Rule)
	r.pdf.SetLineWidth(rowRuleWidth)
	r.pdf.Line(r.left, y, r.right, y)
}

func (r *renderer) startContinuationPage(layout tableLayout) error {
	r.pdf.AddPage()
	if err := r.drawContinuationHeader(); err != nil {
		return err
	}
	return r.drawTableHeader(layout)
}

func (r *renderer) totalLabel() string {
	if r.isCreditNote() || r.doc.Paid {
		return r.labels.totalSettled
	}
	return r.labels.totalDue
}

func (r *renderer) drawTotals() error {
	rows := r.totalRows()
	qr, err := r.paymentQRLayout()
	if err != nil {
		return err
	}
	r.y += totalsAdvance
	needed := float64(len(rows))*totalsRowHeight + totalsMinHeight
	if qr != nil && qr.height > needed {
		needed = qr.height
	}
	if err := r.ensureSpace(needed); err != nil {
		return err
	}
	totalsY := r.y
	if err := r.drawTotalRows(rows); err != nil {
		return err
	}

	x := r.right - totalsBoxWidth
	r.setStrokeColor(r.palette.Accent)
	r.pdf.SetLineWidth(totalsRuleWidth)
	r.pdf.Line(x, r.y, r.right, r.y)
	r.y += totalsRuleOffset

	r.setFillColor(r.palette.Primary)
	r.pdf.RectFromUpperLeftWithStyle(x, r.y, totalsBoxWidth, totalsBoxHeight, "F")
	if err := r.drawText(
		r.totalLabel(),
		x+totalsLabelX,
		r.y+totalsLabelY,
		totalsLabelWidth,
		true,
		totalsLabelSize,
		r.palette.Surface,
		gopdf.Left,
	); err != nil {
		return err
	}
	if err := r.drawText(
		r.formatMoney(r.doc.TotalMinor),
		x+totalsBoxWidth-totalsValueWidth,
		r.y+totalsAmountY,
		totalsValueWidth-totalsAmountInset,
		true,
		totalsAmountSize,
		r.palette.Surface,
		gopdf.Right,
	); err != nil {
		return err
	}
	r.y += totalsBoxHeight + totalsGap

	if qr == nil {
		return nil
	}
	if err = r.drawPaymentQR(qr, r.left, totalsY); err != nil {
		return err
	}
	if qrBottom := totalsY + qr.height + totalsGap; qrBottom > r.y {
		r.y = qrBottom
	}
	return nil
}

func (r *renderer) paymentQRLayout() (*paymentQRLayout, error) {
	qr := r.doc.PaymentQR
	if qr == nil {
		return nil, nil
	}
	size := paymentQRSize
	layout := &paymentQRLayout{png: qr.PNG, size: size, height: size}
	label := strings.TrimSpace(qr.Label)
	if label == "" {
		return layout, nil
	}
	lines, err := r.wrapText(label, size, false, paymentQRLabelSize)
	if err != nil {
		return nil, err
	}
	layout.lines = lines
	layout.height += paymentQRLabelGap + float64(len(lines))*paymentQRLabelLineHeight
	return layout, nil
}

func (r *renderer) drawPaymentQR(layout *paymentQRLayout, x, y float64) error {
	holder, err := gopdf.ImageHolderByBytes(layout.png)
	if err != nil {
		return fmt.Errorf("invoicepdf: payment qr: %w", err)
	}
	rect := &gopdf.Rect{W: layout.size, H: layout.size}
	if err = r.pdf.ImageByHolder(holder, x, y, rect); err != nil {
		return fmt.Errorf("invoicepdf: draw payment qr: %w", err)
	}
	labelY := y + layout.size + paymentQRLabelGap
	for _, line := range layout.lines {
		if err = r.drawText(
			line,
			x,
			labelY,
			layout.size,
			false,
			paymentQRLabelSize,
			r.palette.Muted,
			gopdf.Center,
		); err != nil {
			return err
		}
		labelY += paymentQRLabelLineHeight
	}
	return nil
}

func (r *renderer) totalRows() []totalRow {
	rows := []totalRow{{r.labels.itemsTotal, r.doc.ItemsMinor}}
	if r.doc.ShippingMinor != 0 {
		rows = append(rows, totalRow{r.labels.shipping, r.doc.ShippingMinor})
	}
	if r.doc.DiscountMinor != 0 {
		rows = append(rows, totalRow{r.labels.discount, r.doc.DiscountMinor})
	}
	return rows
}

func (r *renderer) drawTotalRows(rows []totalRow) error {
	x := r.right - totalsBoxWidth
	for _, row := range rows {
		if err := r.drawText(
			row.label,
			x,
			r.y,
			totalsLabelWidth,
			false,
			totalsRowSize,
			r.palette.Muted,
			gopdf.Left,
		); err != nil {
			return err
		}
		if err := r.drawText(
			r.formatMoney(row.value),
			x+totalsBoxWidth-totalsValueWidth,
			r.y,
			totalsValueWidth,
			false,
			totalsRowSize,
			r.palette.Text,
			gopdf.Right,
		); err != nil {
			return err
		}
		r.y += totalsRowHeight
	}
	return nil
}

func (r *renderer) drawPaymentNotice() error {
	if r.isCreditNote() {
		return nil
	}
	label, color, fill := r.paymentNoticeStyle()
	if err := r.ensureSpace(noticeHeight + noticeGap); err != nil {
		return err
	}
	r.setFillColor(fill)
	r.pdf.RectFromUpperLeftWithStyle(r.left, r.y, r.contentWidth(), noticeHeight, "F")
	r.setStrokeColor(color)
	r.pdf.SetLineWidth(noticeRuleWidth)
	r.pdf.RectFromUpperLeftWithStyle(r.left, r.y, r.contentWidth(), noticeHeight, "D")
	r.setFillColor(color)
	r.pdf.RectFromUpperLeftWithStyle(r.left, r.y, noticeAccentWidth, noticeHeight, "F")
	if err := r.drawText(
		label,
		r.left+noticeTextX,
		r.y+noticeTextY,
		r.contentWidth()-noticeTextInset,
		true,
		noticeTextSize,
		color,
		gopdf.Left,
	); err != nil {
		return err
	}
	r.y += noticeHeight + noticeGap
	return nil
}

func (r *renderer) paymentNoticeStyle() (string, Color, Color) {
	if r.doc.Paid {
		return r.labels.paidNotice, r.palette.Primary, r.palette.Surface
	}
	return r.labels.unpaid, r.palette.Accent, r.palette.White
}

func (r *renderer) drawNotes() error {
	if err := r.drawReasonNote(); err != nil {
		return err
	}
	return r.drawTaxNote()
}

func (r *renderer) drawReasonNote() error {
	if !r.isCreditNote() {
		return nil
	}
	reason := strings.TrimSpace(r.doc.Reason)
	if reason == "" {
		return nil
	}
	if err := r.drawReasonHeading(); err != nil {
		return err
	}
	return r.drawReasonText(reason)
}

func (r *renderer) drawReasonHeading() error {
	if err := r.ensureSpace(reasonSpace); err != nil {
		return err
	}
	if err := r.drawText(
		r.labels.reason,
		r.left,
		r.y,
		r.contentWidth(),
		true,
		reasonHeadingSize,
		r.palette.Accent,
		gopdf.Left,
	); err != nil {
		return err
	}
	r.y += reasonHeadingGap
	return nil
}

func (r *renderer) drawReasonText(reason string) error {
	lines, err := r.wrapText(reason, r.contentWidth(), false, reasonSize)
	if err != nil {
		return err
	}
	if err = r.drawWrappedLines(
		lines,
		false,
		reasonSize,
		reasonLineHeight,
		r.palette.Text,
	); err != nil {
		return err
	}
	r.y += reasonGap
	return nil
}

func (r *renderer) drawTaxNote() error {
	if err := r.ensureSpace(taxNoteSpace); err != nil {
		return err
	}
	return r.drawText(
		r.labels.notVATPayer,
		r.left,
		r.y+taxNoteY,
		r.contentWidth(),
		false,
		taxNoteSize,
		r.palette.Muted,
		gopdf.Left,
	)
}

func (r *renderer) drawFooters() error {
	total := r.pdf.GetNumberOfPages()
	if total < 1 {
		return nil
	}
	footerY := r.height - footerRuleOffset
	for page := 1; page <= total; page++ {
		if err := r.pdf.SetPage(page); err != nil {
			return err
		}
		r.setStrokeColor(r.palette.Rule)
		r.pdf.SetLineWidth(footerRuleWidth)
		r.pdf.Line(r.left, footerY, r.right, footerY)
		pageLabel := fmt.Sprintf(pageCountFormat, r.labels.page, page, total)
		if err := r.drawText(
			pageLabel,
			r.left,
			footerY+footerTextY,
			r.contentWidth(),
			false,
			footerSize,
			r.palette.Muted,
			gopdf.Right,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) formatMoney(minor int64) string {
	amount := formatMinorAmount(minor, r.czech)
	currency := currencyLabel(r.doc.Currency, r.czech)
	if currency == "" {
		return amount
	}
	return amount + " " + currency
}

func isCzech(locale string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "cs")
}

func currencyLabel(currency string, czech bool) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "":
		return ""
	case "CZK":
		if czech {
			return "Kč"
		}
		return "CZK"
	default:
		return strings.ToUpper(strings.TrimSpace(currency))
	}
}

func formatMinorAmount(minor int64, czech bool) string {
	text, negative := strings.CutPrefix(strconv.FormatInt(minor, 10), "-")
	for len(text) < minorMinLength {
		text = "0" + text
	}
	whole := text[:len(text)-fractionDigits]
	fraction := text[len(text)-fractionDigits:]

	separator := ","
	decimal := "."
	if czech {
		separator = "\u00A0"
		decimal = ","
	}
	result := groupDigits(whole, separator) + decimal + fraction
	if negative {
		return "-" + result
	}
	return result
}

func groupDigits(digits, separator string) string {
	if len(digits) <= groupSize {
		return digits
	}
	var builder strings.Builder
	lead := len(digits) % groupSize
	if lead > 0 {
		builder.WriteString(digits[:lead])
		builder.WriteString(separator)
	}
	for index := lead; index < len(digits); index += groupSize {
		builder.WriteString(digits[index : index+groupSize])
		if index+groupSize < len(digits) {
			builder.WriteString(separator)
		}
	}
	return builder.String()
}

func formatDate(raw string, czech bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, layout := range []string{dateLayoutISO, time.RFC3339} {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		if czech {
			return parsed.Format(dateLayoutCS)
		}
		return parsed.Format(dateLayoutEN)
	}
	return ""
}

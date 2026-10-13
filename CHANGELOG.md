# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- ISDOC 6.0.2 export through `RenderISDOC`: deterministic Czech e-invoice XML
  in the `http://isdoc.cz/namespace/2013` name space for the `Document` model,
  with invoice and credit-note document types, order and original-document
  references, a deterministic document UUID and zero-rate tax totals.
  `ErrInvalidISDOC` reports documents the format cannot hold.
- Exported `Labels` and `LookupLabels`: the Czech and English label set the
  renderer draws, with `ErrUnsupportedLocale` for other locales.
- `cmd/invoicepdf render`: a batch command that converts JSON documents or
  directories of JSON files to PDF and optional ISDOC output, reports per-file
  status on stderr and prints a deterministic JSON summary on stdout.

## [0.2.0] - 2026-10-13

### Added

- Payment QR support through `Document.PaymentQR` and the new `PaymentQR`
  type: a caller-supplied PNG renders next to the totals with an optional
  centered label.
- Payment QR input validation with descriptive errors for empty or invalid
  PNG data.

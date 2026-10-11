# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-10-13

### Added

- Payment QR support through `Document.PaymentQR` and the new `PaymentQR`
  type: a caller-supplied PNG renders next to the totals with an optional
  centered label.
- Payment QR input validation with descriptive errors for empty or invalid
  PNG data.

// Command invoicepdf renders invoice documents from JSON files to PDF and,
// with --isdoc, to ISDOC 6.0.2 XML. The input schema and the usage are
// documented in the package README.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

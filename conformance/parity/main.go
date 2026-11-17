// Command parity compares the rules that this package raises with the ones
// raised by the reference validators, invoice by invoice: the KoSIT validator
// with the XRechnung configuration, and Mustang. See ../README.md.
//
//	parity stage -corpus DIR -kosit DIR -work DIR
//	parity compare -work DIR [-known known.txt] [-md report.md] [-json report.json]
//
// stage copies the invoices of the ZUGFeRD corpus and of the KoSIT test suite
// into DIR/in under short unique names (0001.xml, 0002.pdf…), lists them in
// DIR/index.tsv with their origin, and writes the lists of files to give to
// each validator (DIR/kosit.list: the XML files, DIR/mustang.list: all).
//
// compare reads the reports that the validators wrote in DIR/kosit
// (0001-report.xml) and DIR/mustang (0001.xml.xml), validates each invoice
// with this package, and compares the rules raised as errors. A difference
// that known.txt does not justify makes the command exit with status 1.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "stage":
		fs := flag.NewFlagSet("stage", flag.ExitOnError)
		corpus := fs.String("corpus", "", "clone of ZUGFeRD/corpus")
		kosit := fs.String("kosit", "", "clone of itplr-kosit/xrechnung-testsuite")
		work := fs.String("work", "", "work directory")
		fs.Parse(os.Args[2:])
		if *corpus == "" || *kosit == "" || *work == "" {
			usage()
		}
		err = stage(*corpus, *kosit, *work)
	case "compare":
		fs := flag.NewFlagSet("compare", flag.ExitOnError)
		work := fs.String("work", "", "work directory")
		known := fs.String("known", "", "file of the known differences")
		md := fs.String("md", "", "Markdown report")
		js := fs.String("json", "", "JSON report")
		fs.Parse(os.Args[2:])
		if *work == "" {
			usage()
		}
		var ok bool
		ok, err = compare(*work, *known, *md, *js)
		if err == nil && !ok {
			os.Exit(1)
		}
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "parity:", err)
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  parity stage -corpus DIR -kosit DIR -work DIR
  parity compare -work DIR [-known known.txt] [-md report.md] [-json report.json]`)
	os.Exit(2)
}

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ross-spencer/spargo/pkg/spargo"
)

var (
	threshold int
	debug     bool
	csv       bool
	trim      int
)

func init() {
	flag.IntVar(&threshold, "threshold", 0, "turn threshold on to output (sigs no. > threshold)")
	flag.BoolVar(&debug, "debug", false, "turn debug debug on to investigate signatures")
	flag.BoolVar(&csv, "csv", false, "create CSV to investigate signatures")
	flag.IntVar(&trim, "trim", 0, "trim signatures when outputting csv")
}

// p:P31 is an instance of a file format.

var url = "https://query.wikidata.org/sparql"
var query = `
	SELECT DISTINCT ?format ?formatLabel ?puid ?ldd ?extension ?mimetype ?sig ?referenceLabel ?date ?encodingLabel ?offset ?relativityLabel WHERE
	{
	  ?format wdt:P31/wdt:P279* wd:Q235557.
	  OPTIONAL { ?format wdt:P2748 ?puid. }
	  OPTIONAL { ?format wdt:P3266 ?ldd }
	  OPTIONAL { ?format wdt:P1195 ?extension }
	  OPTIONAL { ?format wdt:P1163 ?mimetype }
	  OPTIONAL { ?format wdt:P4152 ?sig }
	  OPTIONAL {
	     ?format p:P4152 ?object.
	     ?object prov:wasDerivedFrom ?provenance.
	     ?provenance pr:P248 ?reference;
	        pr:P813 ?date.
	  }
	  OPTIONAL {
	     ?format p:P4152 ?object.
	     ?object pq:P3294 ?encoding.
	     ?object pq:P4153 ?offset.
	  }
	  OPTIONAL {
	     ?format p:P4152 ?object.
	     ?object pq:P2210 ?relativity.
	  }
	  SERVICE wikibase:label { bd:serviceParam wikibase:language "[AUTO_LANGUAGE], en". }
	}
	order by ?format
`

var wikidataMapping = make(map[string]Wikidata)

const formatField = "format"
const puidField = "puid"
const locField = "ldd"
const extField = "extension"
const mimeField = "mimetype"

func getID(wikidataURI string) string {
	splitURI := strings.Split(wikidataURI, "/")
	return splitURI[len(splitURI)-1]
}

func newSignature(wdRecord map[string]spargo.Item) Signature {
	tmpWD := Signature{}
	tmpWD.Signature = wdRecord["sig"].Value
	tmpWD.Provenance = wdRecord["referenceLabel"].Value
	tmpWD.Date = wdRecord["date"].Value
	tmpWD.Encoding = wdRecord["encodingLabel"].Value
	tmpWD.Relativity = wdRecord["relativityLabel"].Value
	return tmpWD
}

// Create a newRecord with fields from the query sent to Wikidata.
//
//		"format"	<-- Wikidata URI.
//		"formatLabel"	<-- Format name.
//		"puid"	<-- PUID returned by Wikidata.
//		"extension"	<-- Format extension.
//		"mimetype"	<-- MimeType as recorded by Wikidata.
//
//		TODO: Let's begin with a count of Wikidata signatures
//			  A format might have multiple signatures that can be used to
//			  match a record. Signatures might have multiple forms, e.g. Hex,
//			  or PRONOM regular expression.
//
//		"sig"	<-- Signature in Wikidata.
//		"referenceLabel"	<-- Signature provenance.
//		"date"	<-- Date the signature was submitted.
//		"encodingLabel"	<-- Encoding used for a Signature.
//		"offset"	<-- Offset relative to a position in a file.
//		"relativityLabel" 	<-- Direction from which to measure an offset for a signature.
//
func newRecord(wdRecord map[string]spargo.Item) Wikidata {
	sig := false
	if wdRecord["sig"].Value != "" {
		sig = true
	}
	wd := Wikidata{}

	wd.ID = getID(wdRecord["format"].Value)
	wd.Name = wdRecord["formatLabel"].Value
	wd.URI = wdRecord["format"].Value

	wd.PRONOM = append(wd.PRONOM, wdRecord["puid"].Value)
	wd.LOC = append(wd.LOC, wdRecord["ldd"].Value)
	wd.Extension = append(wd.Extension, wdRecord["extension"].Value)
	wd.Mimetype = append(wd.Mimetype, wdRecord["mimetype"].Value)

	if sig == true {
		wd.Signatures = append(wd.Signatures, newSignature(wdRecord))
	}

	return wd
}

func contains(items []string, item string) bool {
	for i := range items {
		if items[i] == item {
			return true
		}
	}
	return false
}

func updateSignatures(wd *Wikidata, wdRecord map[string]spargo.Item) {
	found := false
	for _, s := range wd.Signatures {
		if s.Signature == wdRecord["sig"].Value {
			found = true
		}
	}
	if found == false {
		wd.Signatures = append(wd.Signatures, newSignature(wdRecord))
	}
}

// A format record has some repeating properties. updateRecord manages those
// exceptions and adds them to the list if it doesn't already exist.
func updateRecord(wdRecord map[string]spargo.Item, wd Wikidata) Wikidata {
	if contains(wd.PRONOM, wdRecord[puidField].Value) == false {
		wd.PRONOM = append(wd.PRONOM, wdRecord[puidField].Value)
	}
	if contains(wd.LOC, wdRecord[locField].Value) == false {
		wd.LOC = append(wd.LOC, wdRecord[locField].Value)
	}
	if contains(wd.Extension, wdRecord[extField].Value) == false {
		wd.Extension = append(wd.Extension, wdRecord[extField].Value)
	}
	if contains(wd.Mimetype, wdRecord[mimeField].Value) == false {
		wd.Mimetype = append(wd.Mimetype, wdRecord[mimeField].Value)
	}
	if wdRecord["sig"].Value != "" {
		updateSignatures(&wd, wdRecord)
	}
	return wd
}

func analyseWikidataRecords(summary *Summary) {
	for _, wd := range wikidataMapping {
		if len(wd.Signatures) > 1 {
			summary.MultipleSequences++
			summary.Multiples = append(summary.Multiples, wd.URI)
		}
		for _, signature := range wd.Signatures {
			signature.analyseSignature(summary, wd.URI)
		}
		if len(wd.Signatures) != 0 {
			summary.FormatsWithSignatures++
		}
	}
}

func runSPARQL() []map[string]spargo.Item {
	sparqlMe := spargo.SPARQLClient{}
	sparqlMe.ClientInit(url, query)
	res := sparqlMe.SPARQLGo()
	return res.Results.Bindings
}

func main() {
	flag.Parse()
	results := runSPARQL()
	var summary Summary
	for _, wdRecord := range results {
		id := getID(wdRecord[formatField].Value)
		if wikidataMapping[id].ID == "" {
			wikidataMapping[id] = newRecord(wdRecord)
		} else {
			wikidataMapping[id] = updateRecord(wdRecord, wikidataMapping[id])
		}
	}
	summary.AllSparqlResults = len(results)
	summary.CondensedSparqlResults = len(wikidataMapping)
	analyseWikidataRecords(&summary)
	if debug {
		out := ""
		for _, wd := range wikidataMapping {
			if len(wd.Signatures) > threshold {
				for _, signature := range wd.Signatures {
					if !csv {
						out = fmt.Sprintf("%s%s,", out, signature)
					} else {
						out = fmt.Sprintf("%s%s\n", out, signature.CSV(wd.URI, len(wd.Signatures)))
					}
				}
			}
		}
		if !csv {
			fmt.Fprintf(os.Stdout, "[%s]", strings.Trim(out, ","))
			return
		}
		const header = "uri, count, sig, provenance, date, encoding, relativity"
		fmt.Fprintf(os.Stdout, "%s\n%s", header, out)
	} else {
		fmt.Fprintf(os.Stdout, "%s\n", summary)
	}
}

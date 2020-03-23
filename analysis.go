package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ross-spencer/spargo/pkg/spargo"
)

var url = "https://query.wikidata.org/sparql"
var query = `SELECT DISTINCT ?format ?formatLabel ?puid ?ldd ?extension ?mimetype ?sig ?referenceLabel ?date ?encodingLabel ?offset ?relativityLabel WHERE
{
  ?format wdt:P2748 ?puid.
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
  SERVICE wikibase:label { bd:serviceParam wikibase:language "[AUTO_LANGUAGE],en". }
}
order by ?format`

// Wikidata ... might be commented in Siegfried...
type Wikidata struct {
	ID        string    // Wikidata short name, e.g. Q12345 can be appended to a URI to be dereferenced.
	Name      string    // Name of the format as described in Wikidata.
	URI       string    // URI is the absolute URL in Wikidata terms that can be dereferenced.
	PRONOM    []string  // 1:1 mapping to PRONOM wherever possible.
	LOC       []string  // Library of Congress identifiers.
	Extension []string  // Extension returned by Wikidata.
	Mimetype  []string  // Mimetype as recorded by Wikidata.
	Signature Signature // Signature associated with a record which we will convert to a new Type.
}

// Signature ...
type Signature struct {
	Signature  string // Signature byte sequence.
	Provenance string // Provenance of the signature.
	Date       string // Date the signature was submitted.
	Encoding   string // Signature encoding, e.g. Hexadecimal, ASCII, PRONOM.
}

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

	if sig == false {
		wd.Signature = Signature{}
	} else {
		wd.Signature.Signature = wdRecord["sig"].Value
		wd.Signature.Provenance = wdRecord["referenceLabel"].Value
		wd.Signature.Date = wdRecord["date"].Value
		wd.Signature.Encoding = wdRecord["relativityLabel"].Value
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

// A format record has some repeating properties, updateReocrd manages those
// exceptions.
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
	return wd
}

func countSignatures() int {
	var count int
	for _, v := range wikidataMapping {
		if v.Signature.Signature != "" {
			count++
		}
	}
	return count
}

func runSPARQL() []map[string]spargo.Item {
	sparqlMe := spargo.SPARQLClient{}
	sparqlMe.ClientInit(url, query)
	res := sparqlMe.SPARQLGo()
	return res.Results.Bindings
}

func main() {
	results := runSPARQL()
	fmt.Fprintf(os.Stderr, "Original SPARQL results: %d\n", len(results))
	for _, wdRecord := range results {
		id := getID(wdRecord[formatField].Value)
		if wikidataMapping[id].ID == "" {
			wikidataMapping[id] = newRecord(wdRecord)
		} else {
			wikidataMapping[id] = updateRecord(wdRecord, wikidataMapping[id])
		}
	}
	fmt.Fprintf(os.Stderr, "Condensed SPARQL results: %d\n", len(wikidataMapping))
	fmt.Fprintf(os.Stderr, "Number of anticipated signatures: %d\n", countSignatures())
	fmt.Fprintf(os.Stderr, "Report generation complete...\n")
}

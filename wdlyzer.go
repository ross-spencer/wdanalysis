package main

// Maintain a memory of all the linting warnings and errors that are
// discovered parsing the Wikidata report.
//
// Linting is independent of signature processing but provides
// information about the Wikidata record that is concrete and can be
// fixed up. Linting is recorded in parallel with processing the
// signature and returned in one block that can then be accessed.
//
// All errors are treated as a signal to stop processing the signature.
// The signature will not be added to the final identifier and the
// record will need to be updated in Wikidata itself.
//
// Not all linting issues are 'fatal' but some make it difficult to
// process the signature.
//
//
// Signatures that make it through should all be able to be processed by
// Siegfried. We try and output some summary information about the
// signatures.
//

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

const uriField = "uri"
const formatLabelField = "formatLabel"
const puidField = "puid"
const locField = "ldd"
const extField = "extension"
const mimeField = "mimetype"
const signatureField = "sig"
const offsetField = "offset"
const encodingField = "encodingLabel"
const relativityField = "relativityLabel"
const dateField = "date"
const referenceField = "referenceLabel"

const langTemplate = "<<lang>>"
const lang = "en"

var url = "https://query.wikidata.org/sparql"

// Developer note: There is only a loose coupling between the SPARQL
// variables and the variables used in code. As such, updating this
// query should be done with care.
var query = `
    # Return all file format records from Wikidata.
    #
    select distinct ?uri ?uriLabel ?puid ?extension ?mimetype ?encodingLabel ?referenceLabel ?date ?relativityLabel ?offset ?sig
    where
    {
      ?uri wdt:P31/wdt:P279* wd:Q235557.            # Return records of type File Format.
      optional { ?uri wdt:P2748 ?puid.   }          # PUID is used to map to PRONOM signatures proper.
      optional { ?uri wdt:P1195 ?extension  }
      optional { ?uri wdt:P1163 ?mimetype   }
      # We don't need to require that there is a format identification pattern because
      # we want to be able to provide results for items without them that are mapped to
      # PRONOM anyway.
      optional { ?uri p:P4152 ?object;              # Format identification pattern statement.
        optional { ?object pq:P3294 ?encoding.   }     # We don't always have an encoding.
        optional { ?object ps:P4152 ?sig.        }     # We always have a signature.
        optional { ?object pq:P2210 ?relativity. }     # Relativity to beginning or end of file.
        optional { ?object pq:P4153 ?offset.     }     # Offset relatve to the relativity.

        optional { ?object prov:wasDerivedFrom ?provenance;
           optional { ?provenance pr:P248 ?reference;
                                  pr:P813 ?date.
                    }
        }
      }
      # Wikidata's mechanism to return labels from SPARQL parameters.
      service wikibase:label { bd:serviceParam wikibase:language "[AUTO_LANGUAGE], <<lang>>". }
    }
    order by ?uri
`

// WIKIDATA TODO: Write test for this function...
func getID(wikidataURI string) string {
	splitURI := strings.Split(wikidataURI, "/")
	return splitURI[len(splitURI)-1]
}

// newRecord creates a Wikidata record with the values received from
// Wikidata itself.
func newRecord(wdRecord map[string]spargo.Item, addSigs bool) Wikidata {
	wd := Wikidata{}
	wd.ID = getID(wdRecord[uriField].Value)
	wd.Name = wdRecord[formatLabelField].Value
	wd.URI = wdRecord[uriField].Value
	wd.PRONOM = append(wd.PRONOM, wdRecord[puidField].Value)
	wd.Extension = append(wd.Extension, wdRecord[extField].Value)
	wd.Mimetype = append(wd.Mimetype, wdRecord[mimeField].Value)
	if wdRecord[signatureField].Value != "" {
		if !addSigs {
			// Pre-processing has returned no particular heuristic will
			// help us here and so let's make sure we can report on that
			// at the end, as well as exit early.
			addLinting(wd.URI, heuWDE01)
			wd.disableSignatures = true
			return wd
		}
		sig := Signature{}
		wd.Signatures = append(wd.Signatures, sig)
		bs := newByteSequence(wdRecord)
		wd.Signatures[0].ByteSequences = append(wd.Signatures[0].ByteSequences, bs)
	}
	return wd
}

// updateRecord manages a format record's repeating properties.
// exceptions and adds them to the list if it doesn't already exist.
func updateRecord(wdRecord map[string]spargo.Item, wd Wikidata) Wikidata {
	if contains(wd.PRONOM, wdRecord[puidField].Value) == false {
		wd.PRONOM = append(wd.PRONOM, wdRecord[puidField].Value)
	}
	if contains(wd.Extension, wdRecord[extField].Value) == false {
		wd.Extension = append(wd.Extension, wdRecord[extField].Value)
	}
	if contains(wd.Mimetype, wdRecord[mimeField].Value) == false {
		wd.Mimetype = append(wd.Mimetype, wdRecord[mimeField].Value)
	}
	if wdRecord[signatureField].Value != "" {
		if !wd.disableSignatures {
			lintingErr := updateSequences(wdRecord, &wd)
			// WIKIDATA FUTURE: If we can re-organize the signatures in
			// Wikidata so that they are better encapsulated from each
			// other then we don't need to be as strict about not
			// processing the value. Right now, there's not enough
			// consistency in records that mix signatures with multiple
			// sequences, types, offsets and so forth.
			if lintingErr != nle {
				wd.Signatures = nil
				wd.disableSignatures = true
				addLinting(wd.URI, lintingErr)
			}
		}
	}
	return wd
}

// contains will look for the appearance of a string  item in slice of
// strings items.
func contains(items []string, item string) bool {
	for i := range items {
		if items[i] == item {
			return true
		}
	}
	return false
}

// analyseWikidataRecords ...
func analyseWikidataRecords(summary *Summary) {
	recordsWithLinting, allLinting, badHeuristics := countLintingErrors()
	summary.RecordsWithLintingErrors = recordsWithLinting
	summary.AllLintingMessageCount = allLinting
	summary.FormatsWithBadHeuristics = badHeuristics
	for _, wd := range wikidataMapping {
		if len(wd.Signatures) > 0 {
			summary.RecordsWithSignatures++
		}
		for _, sigs := range wd.Signatures {
			if len(sigs.ByteSequences) > 1 {
				summary.MultipleSequences++
			}
		}
	}
}

// runSPARQL ...
func runSPARQL() []map[string]spargo.Item {
	sparqlMe := spargo.SPARQLClient{}
	sparqlMe.ClientInit(url, strings.Replace(query, langTemplate, lang, 1))
	res := sparqlMe.SPARQLGo()
	f, _ := os.Create("res.json")
	defer f.Close()
	f.Write([]byte(res.Human))
	return res.Results.Bindings
}

func main() {
	flag.Parse()
	results := runSPARQL()
	var summary Summary

	var expectedRecordsWithSignatures = make(map[string]bool)
	var allRecordsInclusive = make(map[string]bool)

	for _, wdRecord := range results {
		allRecordsInclusive[wdRecord[uriField].Value] = true
		id := getID(wdRecord[uriField].Value)
		if wdRecord[signatureField].Value != "" {
			summary.SparqlRowsWithSigs++
			expectedRecordsWithSignatures[wdRecord[uriField].Value] = true
		}
		if wikidataMapping[id].ID == "" {
			add := addSignatures(results, id)
			wikidataMapping[id] = newRecord(wdRecord, add)
		} else {
			wikidataMapping[id] = updateRecord(wdRecord, wikidataMapping[id])
		}
	}

	summary.AllSparqlResults = len(results)
	summary.CondensedSparqlResults = len(wikidataMapping)
	summary.RecordsWithPotentialSignatures = len(expectedRecordsWithSignatures)
	analyseWikidataRecords(&summary)

	// WIKIDATA TODO: Flag to show linting errors specific to Wikidata.
	if debug {
		summary.AllLintingMessages = lintingToString()
	}
	fmt.Fprintf(os.Stdout, "%s\n", summary)
}

package main

import (
	"encoding/json"
	"fmt"
)

// Wikidata ... might be commented in Siegfried...
type Wikidata struct {
	ID         string      // Wikidata short name, e.g. Q12345 can be appended to a URI to be dereferenced.
	Name       string      // Name of the format as described in Wikidata.
	URI        string      // URI is the absolute URL in Wikidata terms that can be dereferenced.
	PRONOM     []string    // 1:1 mapping to PRONOM wherever possible.
	LOC        []string    // Library of Congress identifiers.
	Extension  []string    // Extension returned by Wikidata.
	Mimetype   []string    // Mimetype as recorded by Wikidata.
	Signatures []Signature // Signature associated with a record which we will convert to a new Type.
}

// Signature ...
type Signature struct {
	Signature  string // Signature byte sequence.
	Provenance string // Provenance of the signature.
	Date       string // Date the signature was submitted.
	Encoding   string // Signature encoding, e.g. Hexadecimal, ASCII, PRONOM.
	Relativity string // Position relative to beginning or end of file, or elsewhere.
}

// Serialize the signature component of our record to a string to debug.
func (s Signature) String() string {
	report, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s", report)
}

// CSV will serialize the signature component of our record to a csv to debug.
func (s Signature) CSV(uri string, count int) string {
	provenance := s.Provenance
	date := s.Date
	encoding := s.Encoding
	relativity := s.Relativity
	signature := s.Signature
	if provenance == "" {
		provenance = "None"
	}
	if date == "" {
		date = "None"
	}
	if encoding == "" {
		encoding = "None"
	}
	if relativity == "" {
		relativity = "None"
	}
	if len(signature) >= trim && trim > 0 {
		signature = s.Signature[:trim]
	}
	return fmt.Sprintf("%s, %d, %s, %s, %s, %s, %s",
		uri,
		count,
		signature,
		provenance,
		date,
		encoding,
		relativity,
	)
}

var enc = false

func (s Signature) analyseSignature(summary *Summary, uri string) {
	if s.Provenance == "" {
		summary.ErrNoProvenance++
		if uri != "" && !contains(summary.NoProvenance, uri) {
			summary.NoProvenance = append(summary.NoProvenance, uri)
		}
	}
	if s.Date == "" {
		summary.ErrNoDate++
		if uri != "" && !contains(summary.NoDate, uri) {
			summary.NoDate = append(summary.NoDate, uri)
		}
	}
	if s.Encoding == "" {
		summary.ErrNoEncoding++
		if uri != "" && !contains(summary.NoEncoding, uri) {
			summary.NoEncoding = append(summary.NoEncoding, uri)
		}
		if !enc && !contains(summary.EncodingSet, "None") {
			summary.EncodingSet = append(summary.EncodingSet, "None")
		}
	} else {
		if !contains(summary.EncodingSet, s.Encoding) {
			summary.EncodingSet = append(summary.EncodingSet, s.Encoding)
		}
	}
	if s.Relativity == "" {
		summary.ErrNoRelativity++
		if uri != "" && !contains(summary.NoRelativity, uri) {
			summary.NoRelativity = append(summary.NoRelativity, uri)
		}
	}
}

// Summary of the identifier.
type Summary struct {
	AllSparqlResults       int
	CondensedSparqlResults int
	FormatsWithSignatures  int
	MultipleSequences      int
	ErrNoProvenance        int
	ErrNoDate              int
	ErrNoRelativity        int
	ErrNoEncoding          int

	// Sets to help understand content.
	EncodingSet []string

	// Records that need investigating.
	Multiples    []string
	NoProvenance []string
	NoDate       []string
	NoRelativity []string
	NoEncoding   []string
}

// String will return a summary report to be printed.
func (s Summary) String() string {
	report, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s", report)
}

package main

import (
	"encoding/json"
	"fmt"
)

// Wikidata ... might be commented in Siegfried...
type Wikidata struct {
	ID                string      // Wikidata short name, e.g. Q12345 can be appended to a URI to be dereferenced.
	Name              string      // Name of the format as described in Wikidata.
	URI               string      // URI is the absolute URL in Wikidata terms that can be dereferenced.
	PRONOM            []string    // 1:1 mapping to PRONOM wherever possible.
	Extension         []string    // Extension returned by Wikidata.
	Mimetype          []string    // Mimetype as recorded by Wikidata.
	Signatures        []Signature // Signature associated with a record which we will convert to a new Type.
	disableSignatures bool        // If a bad heuristic was found we can't reliably add signatures to the record.
}

// Signature ...
type Signature struct {
	ByteSequences []ByteSequence // A signature is made up of multiple byte sequences that encode a position and a pattern, e.g. BOF and EOF.
}

// ByteSequence ...
type ByteSequence struct {
	Signature  string // Signature byte sequence.
	Offset     int    // Offset used by the signature.
	Provenance string // Provenance of the signature.
	Date       string // Date the signature was submitted.
	Encoding   int    // Signature encoding, e.g. Hexadecimal, ASCII, PRONOM.
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

// Serialize the byte sequence component of our record to a string to
// debug.
func (b ByteSequence) String() string {
	report, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s", report)
}

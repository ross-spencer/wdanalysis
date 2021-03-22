package main

import (
	"encoding/json"
	"fmt"
)

// Summary of the identifier.
type Summary struct {
	AllSparqlResults               int      // All rows of data returned from our SPARQL request.
	CondensedSparqlResults         int      // All unique records once the SPARQL is processed.
	SparqlRowsWithSigs             int      // All SPARQL rows with signatures (SPARQL necessarily returns duplicates).
	RecordsWithPotentialSignatures int      // Records that have signatures that can be processed.
	FormatsWithBadHeuristics       int      // Formats that have bad heuristics that we can't process.
	RecordsWithSignatures          int      // Records remaining that were processed.
	MultipleSequences              int      // Records that have been parsed out into multiple signatures per record.
	AllLintingMessages             []string // All linting messages returned.
	AllLintingMessageCount         int      // Count of all linting messages output.
	RecordsWithLintingErrors       int      // Records that have linting errors that we can fix.
}

// String will return a summary report to be printed.
func (s Summary) String() string {
	report, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s", report)
}

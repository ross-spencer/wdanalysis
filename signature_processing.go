package main

import (
	"github.com/ross-spencer/spargo/pkg/spargo"
	"github.com/ross-spencer/wdlyzer/pkg/converter"
)

// handleLinting ensures that our sequence error arrays are added to
// following the validation of the information.
func handleLinting(uri string, lint linting) {
	if lint != nle {
		addLinting(uri, lint)
	}
}

// newSignature will parse signature information from the Spargo Item
// structure and create a new Signature structure to be returned. If
// there is an error we log it out with the format identifier so that
// more work can be done on the source data.
func newByteSequence(wdRecord map[string]spargo.Item) ByteSequence {

	tmpSequence := ByteSequence{}

	uri := wdRecord[uriField].Value

	// Add provenance source to sequence.
	provenance, lint := validateAndReturnProvenance(wdRecord[referenceField].Value)
	handleLinting(uri, lint)
	tmpSequence.Provenance = provenance

	// Add provenance date to sequence.
	date, lint := validateAndReturnDate(wdRecord[dateField].Value, provenance)
	handleLinting(uri, lint)
	tmpSequence.Date = date

	// Add relativity to sequence.
	relativity, lint, _ := validateAndReturnRelativity(wdRecord[relativityField].Value)
	handleLinting(uri, lint)
	tmpSequence.Relativity = relativity

	// Add offset to sequence.
	offset, lint := validateAndReturnOffset(wdRecord[offsetField].Value, wdRecord[offsetField].Type)
	handleLinting(uri, lint)
	tmpSequence.Offset = offset

	// Add encoding to sequence.
	encoding, lint := validateAndReturnEncoding(wdRecord[encodingField].Value)
	handleLinting(uri, lint)
	tmpSequence.Encoding = encoding

	// Add the signature to the sequence.
	signature, lint, _ := validateAndReturnSignature(wdRecord[signatureField].Value, encoding)
	handleLinting(uri, lint)
	tmpSequence.Signature = signature

	return tmpSequence
}

// updateSignatures will create a new ByteSequence and associate it
// with either an existing Signature or create a brand new Signature.
// If there is a problem processing that means the sequence shouldn't be
// added to the identifier for the sake of consistency then a linting
// error is returned and we should stop processing.
//
// WIKIDATA TODO: Write tests for this.
//
func updateSequences(wdRecord map[string]spargo.Item, wd *Wikidata) linting {

	// Pre-process the encoding.
	encoding, lint := validateAndReturnEncoding(wdRecord[encodingField].Value)
	handleLinting(wd.URI, lint)

	// Pre-process the relativity.
	relativity, lint, _ := validateAndReturnRelativity(wdRecord[relativityField].Value)
	handleLinting(wd.URI, lint)

	// Pre-process the sequence.
	signature, lint, _ := validateAndReturnSignature(wdRecord[signatureField].Value, encoding)
	handleLinting(wd.URI, lint)

	// WIKIDATA FUTURE it's nearly impossible to tease apart sequences in
	// Wikidata right now to determine which duplicate sequences are new
	// signatures or which belong to the same group. Provenance could differ
	// but three can be multiple provenances, different sequences which they're
	// returned from the service, etc.
	if !sequenceInSignatures(wd.Signatures, signature) {
		if relativityAlreadyInSignatures(wd.Signatures, relativity) {
			if relativity == relativeBOF {
				// Create a new record...
				sig := Signature{}
				bs := newByteSequence(wdRecord)
				sig.ByteSequences = append(sig.ByteSequences, bs)
				wd.Signatures = append(wd.Signatures, sig)
				return nle
			} else {
				// We've a bad heuristic and can't piece together a
				// valid signature.
				return heuWDE01
			}
		} else {
			// Append to record...
			idx := len(wd.Signatures)
			sig := &wd.Signatures[idx-1]
			if checkEncodingCompatibility(wd.Signatures[idx-1], encoding) {
				bs := newByteSequence(wdRecord)
				sig.ByteSequences = append(sig.ByteSequences, bs)
				return nle
			} else {
				// We've a bad heuristic and can't piece together a
				// valid signature.
				return heuWDE01
			}
		}
	}
	// Sequence already in signatures, no need to process, no errors of note.
	return nle
}

// sequenceInSignatures will tell us if there are any duplicate byte
// sequences. At which point we can stop processing.
func sequenceInSignatures(signatures []Signature, signature string) bool {
	for _, sig := range signatures {
		for _, seq := range sig.ByteSequences {
			if signature == seq.Signature {
				return true
			}
		}
	}
	return false
}

// relativityInSlice ...
func relativityAlreadyInSignatures(signatures []Signature, relativity string) bool {
	for _, sig := range signatures {
		for _, seq := range sig.ByteSequences {
			if relativity == seq.Relativity {
				return true
			}
		}
	}
	return false
}

// checkEncodingCompatibility should work for now and just makes sure we're not
// trying to combine encodings that don't match, i.e. anything not PRONOM or
// HEX. ASCII should work too because we'll have encoded it as hex by now 🤞.
func checkEncodingCompatibility(signature Signature, givenEncoding int) bool {
	for _, seq := range signature.ByteSequences {
		if (seq.Encoding == converter.GUIDEncoding && givenEncoding != converter.GUIDEncoding) ||
			(seq.Encoding == converter.PerlEncoding && givenEncoding != converter.PerlEncoding) {
			return false
		}
	}
	return true
}

// preValidateSignatures ...
func preValidateSignatures(preProcessedSequences []preProcessedSequence) bool {
	// Map our values into slices to analyze cross-sectionally.
	var encoding []string
	var relativity []string
	var offset []string
	var signature []string
	for _, v := range preProcessedSequences {
		encoding = append(encoding, v.encoding)
		if v.relativity != "" {
			relativity = append(relativity, v.relativity)
		}
		offset = append(offset, v.offset)
		signature = append(signature, v.signature)
		_, _, err := validateAndReturnRelativity(v.relativity)
		if err != nil {
			return false
		}
		_, _, err = validateAndReturnSignature(v.signature, converter.LookupEncoding(v.encoding))
		if err != nil {
			return false
		}
	}
	// Maps act like sets when we're only interested in the keys. We
	// want to use sets to understand more about the unique values in
	// each of the records.
	var relativityMap = make(map[string]bool)
	var signatureMap = make(map[string]bool)
	var encodingMap = make(map[string]bool)
	for _, v := range signature {
		signatureMap[v] = true
	}
	for _, v := range relativity {
		relativityMap[v] = true
	}
	for _, v := range encoding {
		encodingMap[v] = true
	}
	if len(preProcessedSequences) == 2 {
		// The most simple validation we can do. If both we have two
		// values and two different relativities we can let the
		// signature through.
		if len(relativityMap) == 2 {
			return true
		}
		// If the relativities don't differ or aren't available then we
		// can then check to see if the signatures are different
		// because we will create two new records the the sequences.
		// They will both be beginning of file sequences.
		if len(signatureMap) == 2 {
			return true
		}
	}
	// We are going to start wrestling with a sensible heuristic with
	// sequences over 2 in length. Validate those.
	if len(preProcessedSequences) > 2 {
		// Processing starts to get too complicated if we have to work
		// out whether multiple encodings are valid when combined.
		if len(encodingMap) != 1 && len(encodingMap) != 0 {
			return false
		}
		// If we haven't a uniform relativity then we can't easily
		// guess how to combine signatures, e.g. how do we pair a single
		// EOF with one of three BOF sequences? Albeit an unlikely
		// scenario. but also, What if the EOF was not meant to be
		// paired?
		if len(relativityMap) != 1 && len(relativityMap) != 0 {
			return false
		}

	}
	// We should have enough information in these records to be able to
	// write a signature that is reliable.
	if len(signature) == len(encoding) && len(offset) == len(signature) {
		if len(relativity) == 0 || len(relativity) == len(signature) {
			return true
		}

	}
	// Anything else, we can't guarantee enough about the sequences to
	// write a signature. We may still have issues with the one's we've
	// pre-processed even, but we can give ourselves a chance.
	return false
}

// addSignatures ...
func addSignatures(wdRecords []map[string]spargo.Item, id string) bool {
	var preProcessedSequences []preProcessedSequence
	for _, wdRecord := range wdRecords {
		if getID(wdRecord[uriField].Value) == id {
			if wdRecord[signatureField].Value != "" {
				preProcessed := preProcessedSequence{}
				preProcessed.signature = wdRecord[signatureField].Value
				preProcessed.offset = wdRecord[offsetField].Value
				preProcessed.encoding = wdRecord[encodingField].Value
				preProcessed.relativity = wdRecord[relativityField].Value
				if len(preProcessedSequences) == 0 {
					preProcessedSequences = append(preProcessedSequences, preProcessed)
				}
				found := false
				for _, v := range preProcessedSequences {
					if preProcessed == v {
						found = true
						break
					}
				}
				if !found {
					preProcessedSequences = append(preProcessedSequences, preProcessed)
				}
			}
		}
	}
	var add bool
	if len(preProcessedSequences) > 0 {
		add = preValidateSignatures(preProcessedSequences)
	}
	return add
}

package binder

// utf8BOM is the three-byte UTF-8 byte-order mark (U+FEFF).
const utf8BOM = "\xEF\xBB\xBF"

// Serialize reconstructs the original source bytes from a ParseResult.
// The output is byte-identical to the input passed to Parse for all valid inputs.
func Serialize(r *ParseResult) []byte {
	buf := []byte{}
	if r.HasBOM {
		buf = append(buf, utf8BOM...)
	}
	for i, line := range r.Lines {
		buf = append(buf, line...)
		buf = append(buf, r.LineEnds[i]...)
	}
	return buf
}

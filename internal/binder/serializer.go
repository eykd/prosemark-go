package binder

// Serialize reconstructs the original source bytes from a ParseResult.
// The output is byte-identical to the input passed to Parse for all valid inputs.
func Serialize(r *ParseResult) []byte {
	buf := []byte{}
	if r.HasBOM {
		buf = append(buf, 0xEF, 0xBB, 0xBF)
	}
	for i, line := range r.Lines {
		buf = append(buf, line...)
		buf = append(buf, r.LineEnds[i]...)
	}
	return buf
}

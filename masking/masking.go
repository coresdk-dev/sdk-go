package masking

// MaskString redacts PII patterns from s, returning "[REDACTED]" if any PII is found.
// For partial redaction of structured strings, use MaskValue directly.
func MaskString(s string) string {
	return MaskValue(s)
}

// MaskMap redacts PII from every value in m. Keys are checked against blockedFields.
// Returns a new map; the input is not modified.
func MaskMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if IsBlockedField(k) {
			out[k] = redacted
		} else {
			out[k] = MaskValue(v)
		}
	}
	return out
}

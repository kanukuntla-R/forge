package cli

// plural returns singular when n == 1, plural otherwise.
func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}

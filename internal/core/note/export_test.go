package note

// IsPureRenoteForTest exposes isPureRenote to package-external tests.
func IsPureRenoteForTest(in CreateInput) bool {
	return isPureRenote(in)
}

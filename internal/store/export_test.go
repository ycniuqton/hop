package store

// ErrorTranslatorForTest exposes the S3 error mapping to external tests.
func ErrorTranslatorForTest(bucket string) func(Op, string, error) error {
	return errorTranslator{bucket: bucket}.translate
}

package KCES

func testStringPointer(value string) *string {
	return &value
}

func testStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func testStringValues(values []*string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = testStringValue(value)
	}
	return result
}

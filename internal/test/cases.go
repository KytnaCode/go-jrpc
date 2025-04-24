package test

// Aspect represents an aspect of a test case that can have multiple values.
type Aspect []AspectValue

// AspectValue represents a single value for an aspect of a test case.
type AspectValue struct {
	key      string     // The key used to identify the value in the test case.
	name     string     // The name of the value, used for generating test case names.
	value    any        // The actual value of the aspect.
	valueGen func() any // If provided, a function to generate the value dynamically.
}

// genTestCases generates all combinations of test cases based on the provided aspects and values.
func genTestCases(
	name string,
	values []AspectValue,
	cases map[string]map[string]any,
	aspects ...Aspect,
) {
	if len(aspects) == 0 {
		cases[name] = make(map[string]any, len(values))

		for _, v := range values {
			val := v.value

			// if generator is provided, use it to generate the value.
			if v.valueGen != nil {
				val = v.valueGen()
			}

			cases[name][v.key] = val
		}

		return
	}

	for _, v := range aspects[0] {
		finalName := name

		// If the name is not empty, append an underscore before adding the new value.
		if finalName != "" {
			finalName += "_"
		}

		finalName += v.name

		// Recursively generate test cases for the remaining aspects.
		genTestCases(finalName, append(values, v), cases, aspects[1:]...)
	}
}

// NewAspect creates a new aspect with the given key and values, for creating values you can use [NewValue] or
// [CreateValue]. aspect value is indexed by the provided key in the test case map.
func NewAspect(key string, values ...AspectValue) Aspect {
	for i := range values {
		values[i].key = key
	}

	return values
}

// NewValue creates a new aspect value with the given name and value. The value is used directly in the test case.
// If you need to generate the value dynamically, for example, to create a new channel for each tests, use
// [CreateValue] instead.
func NewValue(name string, value any) AspectValue {
	return AspectValue{
		name:  name,
		value: value,
	}
}

// CreateValue creates a new aspect value with the given name and a generator function. The generator function is
// called to generate a new value each time the test case is generated. This is useful for creating values that don't
// should be shared between test cases, like channels.
func CreateValue(name string, generator func() any) AspectValue {
	return AspectValue{
		name:     name,
		valueGen: generator,
	}
}

// GenTestCases generates all combinations of test cases based on the provided aspects and values, and returns them as
// a map where the keys are the test case names, and the values are maps of aspect keys to their corresponding values.
func GenTestCases(aspects ...Aspect) map[string]map[string]any {
	n := len(aspects)
	comb := 1

	testCases := make(map[string]map[string]any, comb)
	values := make([]AspectValue, 0, n)

	genTestCases("", values, testCases, aspects...)

	return testCases
}

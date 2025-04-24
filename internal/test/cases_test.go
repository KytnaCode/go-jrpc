package test_test

import (
	"testing"

	"github.com/kytnacode/go-jrpc/internal/test"
)

const (
	nilResult    = "nil-result"
	floatParams  = "float-params"
	integerError = "integer-error"
)

const (
	resultKey = "result"
	argsKey   = "args"
	errorKey  = "error"
)

func genAspects() []test.Aspect {
	return []test.Aspect{
		test.NewAspect(resultKey,
			test.NewValue(nilResult, nil),
			test.NewValue("integer-result", 10),
			test.CreateValue("float-result", func() any { return 4.0 }),
		),
		test.NewAspect(argsKey,
			test.NewValue("nil-params", nil),
			test.NewValue(floatParams, func() any { return 4.0 }),
		),
		test.NewAspect(errorKey,
			test.NewValue("nil-error", nil),
			test.NewValue(integerError, 10),
			test.CreateValue("float-error", func() any { return 4.0 }),
		),
	}
}

func TestGenTestCases_ShouldGenerateAllCombinations(t *testing.T) {
	t.Parallel()

	type testData struct {
		comb    int
		aspects []test.Aspect
	}

	aspects := genAspects()

	resultAsp, argsAsp, errAsp := aspects[0], aspects[1], aspects[2]

	tests := map[string]testData{
		"no-aspects": {
			comb:    1,
			aspects: []test.Aspect{},
		},
		"one-aspect": {
			comb:    3,
			aspects: []test.Aspect{resultAsp},
		},
		"two-aspects": {
			comb:    3 * 2,
			aspects: []test.Aspect{resultAsp, argsAsp},
		},
		"three-aspects": {
			comb:    3 * 2 * 3,
			aspects: []test.Aspect{resultAsp, argsAsp, errAsp},
		},
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cases := test.GenTestCases(data.aspects...)

			if len(cases) != data.comb {
				t.Errorf("Expected %d combinations, got %d", data.comb, len(cases))
			}
		})
	}
}

func TestGenTestCases_ShouldGenerateCorrectNames(t *testing.T) {
	t.Parallel()

	type testData struct {
		name    string
		aspects []test.Aspect
	}

	aspects := genAspects()

	resultAsp, argsAsp, errAsp := aspects[0], aspects[1], aspects[2]

	tests := map[string]testData{
		"no-aspects": {
			name:    "",
			aspects: []test.Aspect{},
		},
		"one-aspect": {
			name:    nilResult,
			aspects: []test.Aspect{resultAsp},
		},
		"two-aspects": {
			name:    nilResult + "_" + floatParams,
			aspects: []test.Aspect{resultAsp, argsAsp},
		},
		"three-aspects": {
			name:    nilResult + "_" + floatParams + "_" + integerError,
			aspects: []test.Aspect{resultAsp, argsAsp, errAsp},
		},
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cases := test.GenTestCases(data.aspects...)

			if _, ok := cases[data.name]; !ok {
				t.Errorf("Expected case with name '%s' not found", data.name)
			}
		})
	}
}

func TestGenTestCases_ShouldGenerateTestDataWithValuesIndexedWithCorrectKey(t *testing.T) {
	t.Parallel()

	type testData struct {
		aspects []test.Aspect
		keys    []string
	}

	aspects := genAspects()

	resultAsp, argsAsp, errAsp := aspects[0], aspects[1], aspects[2]

	tests := map[string]testData{
		"no-aspects": {
			aspects: []test.Aspect{},
			keys:    []string{},
		},
		"one-aspect": {
			aspects: []test.Aspect{resultAsp},
			keys:    []string{resultKey},
		},
		"two-aspects": {
			aspects: []test.Aspect{resultAsp, argsAsp},
			keys:    []string{resultKey, argsKey},
		},
		"three-aspects": {
			aspects: []test.Aspect{resultAsp, argsAsp, errAsp},
			keys:    []string{resultKey, argsKey, errorKey},
		},
	}

	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cases := test.GenTestCases(data.aspects...)

			for _, testCase := range cases {
				for _, key := range data.keys {
					if _, ok := testCase[key]; !ok {
						t.Errorf("Expected key '%s' not found in test case", key)
					}
				}
			}
		})
	}
}

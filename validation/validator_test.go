package validation

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorMap map[string]string

func (em errorMap) AppendErrorField(name string, message string) {
	em[name] = message
}

type TestBasicStruct struct {
	RequiredEmail         string `validation:"required,email"`
	RequiredValidValue    string `validation:"required,values:one|two|three"`
	ValidValue            string `validation:"values:alpha|beta|gamma"`
	InsensitiveValidValue string `validation:"values-insensitive:alpha|beta|gamma"`
	TooShortValue         string `validation:"min-length:3"`
	TooLongValue          string `validation:"max-length:30"`
}

type TestPointerStruct struct {
	RequiredEmail         *string `validation:"required,email"`
	RequiredValidValue    *string `validation:"required,values:one|two|three"`
	ValidValue            *string `validation:"values:alpha|beta|gamma"`
	InsensitiveValidValue *string `validation:"values-insensitive:alpha|beta|gamma"`
	TooShortValue         *string `validation:"min-length:3"`
	TooLongValue          *string `validation:"max-length:30"`
}

func TestStructsSuccess(t *testing.T) {
	email := "test@example.local"
	requiredValidValue := "three"
	validValue := "gamma"
	insensitiveValidValue := "BETA"
	tooShortValue := "foo"
	tooLongValue := "foo"
	structs := setupStructs(
		&email,
		&requiredValidValue,
		&validValue,
		&insensitiveValidValue,
		&tooShortValue,
		&tooLongValue,
	)
	for _, ts := range structs {
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.Nil(t, err, "This struct should have passed validation, instead got: %#v", em)
	}
}

func TestStructsRequiredFailure(t *testing.T) {
	email := ""
	requiredValidValue := ""
	validValue := "gamma"
	insensitiveValidValue := "BETA"
	tooShortValue := "foo"
	tooLongValue := "foo"
	structs := setupStructs(
		&email,
		&requiredValidValue,
		&validValue,
		&insensitiveValidValue,
		&tooShortValue,
		&tooLongValue,
	)
	for _, ts := range structs {
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.NotNil(t, err, "This struct should have failed validation")
		assert.Len(t, em, 2, "This struct should have 2 errors, instead got: %#v", em)
	}
}

func TestStructsRequiredValidValueFailure(t *testing.T) {
	email := "test@example.com"
	requiredValidValue := ""
	validValue := "gamma"
	insensitiveValidValue := "BETA"
	tooShortValue := "foo"
	tooLongValue := "foo"
	structs := setupStructs(
		&email,
		&requiredValidValue,
		&validValue,
		&insensitiveValidValue,
		&tooShortValue,
		&tooLongValue,
	)
	// Test that only the required error is sent back
	for _, ts := range structs {
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.NotNil(t, err, "This struct should have failed validation")
		// This should only get the required error message, and skip the value validation
		assert.Len(t, em, 1, "This struct should have 1 errors, instead got: %#v", em)
		assert.Equal(t, em["RequiredValidValue"], requiredMessage)
	}
	requiredValidValue = "foo"
	structs = setupStructs(
		&email,
		&requiredValidValue,
		&validValue,
		&insensitiveValidValue,
		&tooShortValue,
		&tooLongValue,
	)
	// Test that only the bad value error is sent back
	for _, ts := range structs {
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.NotNil(t, err, "This struct should have failed validation")
		// This should only get the required error message, and skip the value validation
		assert.Len(t, em, 1, "This struct should have 1 errors, instead got: %#v", em)
		assert.Equal(t, em["RequiredValidValue"], fmt.Sprintf(validValueMessage, "one, two, three"))
	}
}

func TestStructsValidEmailFailure(t *testing.T) {
	email := "bad-email"
	requiredValidValue := "one"
	validValue := "gamma"
	insensitiveValidValue := "BETA"
	tooShortValue := "foo"
	tooLongValue := "foo"
	structs := setupStructs(
		&email,
		&requiredValidValue,
		&validValue,
		&insensitiveValidValue,
		&tooShortValue,
		&tooLongValue,
	)
	for _, ts := range structs {
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.NotNil(t, err, "This struct should have failed validation")
		assert.Len(t, em, 1, "This struct should have 2 errors, instead got: %#v", em)
		assert.Equal(t, em["RequiredEmail"], emailMessage)
	}
}

func TestStructsInsensitiveValidValueFailure(t *testing.T) {
	email := "test@example.com"
	requiredValidValue := "one"
	validValue := "gamma"
	insensitiveValidValue := "BET"
	tooShortValue := "foo"
	tooLongValue := "foo"
	structs := setupStructs(
		&email,
		&requiredValidValue,
		&validValue,
		&insensitiveValidValue,
		&tooShortValue,
		&tooLongValue,
	)
	// Test that only the required error is sent back
	for _, ts := range structs {
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.NotNil(t, err, "This struct should have failed validation")
		// This should only get the required error message, and skip the value validation
		assert.Len(t, em, 1, "This struct should have 1 errors, instead got: %#v", em)
		assert.Equal(t, em["InsensitiveValidValue"], fmt.Sprintf(validValueMessage, "alpha, beta, gamma"))
	}
}

func TestStructsTooShortValueFailure(t *testing.T) {
	email := "test@example.com"
	requiredValidValue := "one"
	validValue := "gamma"
	insensitiveValidValue := "Beta"
	tooShortValue := "oo"
	tooLongValue := "foo"
	structs := setupStructs(
		&email,
		&requiredValidValue,
		&validValue,
		&insensitiveValidValue,
		&tooShortValue,
		&tooLongValue,
	)
	// Test that only the required error is sent back
	for _, ts := range structs {
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.NotNil(t, err, "This struct should have failed validation")
		// This should only get the required error message, and skip the value validation
		assert.Len(t, em, 1, "This struct should have 1 errors, instead got: %#v", em)
		assert.Equal(t, em["TooShortValue_too_short"], fmt.Sprintf(tooShortMessage, 3))
	}
}

func TestStructsTooLongValueFailure(t *testing.T) {
	email := "test@example.com"
	requiredValidValue := "one"
	validValue := "gamma"
	insensitiveValidValue := "Beta"
	tooShortValue := "foo"
	tooLongValue := "foo  foo  foo  foo  foo  foo  foo  "
	structs := setupStructs(
		&email,
		&requiredValidValue,
		&validValue,
		&insensitiveValidValue,
		&tooShortValue,
		&tooLongValue,
	)
	// Test that only the required error is sent back
	for _, ts := range structs {
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.NotNil(t, err, "This struct should have failed validation")
		// This should only get the required error message, and skip the value validation
		assert.Len(t, em, 1, "This struct should have 1 errors, instead got: %#v", em)
		assert.Equal(t, em["TooLongValue_too_long"], fmt.Sprintf(tooLongMessage, 30))
	}
}

func TestStructsNotZero(t *testing.T) {
	toInt64Ptr := func(v int64) *int64 { return &v }
	toFloat64Ptr := func(v float64) *float64 { return &v }
	toTimePtr := func(v time.Time) *time.Time { return &v }
	t.Run("Passes when non-zero values are in struct", func(t *testing.T) {
		var ts struct {
			Integer   int64     `validation:"not-zero"`
			Float     float64   `validation:"not-zero"`
			TimeStamp time.Time `validation:"not-zero"`
		}
		em := make(errorMap, 0)
		ts.Integer = 42
		ts.Float = 42.0
		ts.TimeStamp = time.Now()
		err := ValidateStruct(ts, em)
		require.NoError(t, err)

		var ptrStruct struct {
			Integer   *int64     `validation:"not-zero"`
			Float     *float64   `validation:"not-zero"`
			TimeStamp *time.Time `validation:"not-zero"`
		}
		em2 := make(errorMap, 0)
		ptrStruct.Integer = toInt64Ptr(42)
		ptrStruct.Float = toFloat64Ptr(42.0)
		ptrStruct.TimeStamp = toTimePtr(time.Now())
		err2 := ValidateStruct(ptrStruct, em2)
		require.NoError(t, err2)
	})
	t.Run("Fails when zero values are in struct", func(t *testing.T) {
		var ts struct {
			Integer   int64     `validation:"not-zero"`
			Float     float64   `validation:"not-zero"`
			TimeStamp time.Time `validation:"not-zero"`
		}
		em := make(errorMap, 0)
		err := ValidateStruct(ts, em)
		require.Error(t, err)
		for _, v := range em {
			assert.Equal(t, requiredMessage, v)
		}
		var ptrStruct struct {
			Integer   *int64     `validation:"not-zero"`
			Float     *float64   `validation:"not-zero"`
			TimeStamp *time.Time `validation:"not-zero"`
		}
		em2 := make(errorMap, 0)
		err2 := ValidateStruct(ptrStruct, em2)
		require.Error(t, err2)
		for _, v := range em2 {
			assert.Equal(t, requiredMessage, v)
		}
		ptrStruct.Integer = toInt64Ptr(0)
		ptrStruct.Float = toFloat64Ptr(0)
		ptrStruct.TimeStamp = toTimePtr(time.Time{})
		em3 := make(errorMap, 0)
		err3 := ValidateStruct(ptrStruct, em3)
		require.Error(t, err3)
		for _, v := range em3 {
			assert.Equal(t, requiredMessage, v)
		}
	})
}

func setupStructs(email, requiredValidValue, validValue, insensitiveValidValue, tooShortValue, tooLongValue *string) []interface{} {
	var emailString, requiredValidValueString, validValueString, insensitiveValidValueString, tooShortValueString, tooLongValueString string
	if email != nil {
		emailString = *email
	}
	if requiredValidValue != nil {
		requiredValidValueString = *requiredValidValue
	}
	if validValue != nil {
		validValueString = *validValue
	}
	if insensitiveValidValue != nil {
		insensitiveValidValueString = *insensitiveValidValue
	}
	if tooShortValue != nil {
		tooShortValueString = *tooShortValue
	}
	if tooLongValue != nil {
		tooLongValueString = *tooLongValue
	}
	return []interface{}{
		TestPointerStruct{
			RequiredEmail:         email,
			RequiredValidValue:    requiredValidValue,
			ValidValue:            validValue,
			InsensitiveValidValue: insensitiveValidValue,
			TooShortValue:         tooShortValue,
			TooLongValue:          tooLongValue,
		},
		TestBasicStruct{
			RequiredEmail:         emailString,
			RequiredValidValue:    requiredValidValueString,
			ValidValue:            validValueString,
			InsensitiveValidValue: insensitiveValidValueString,
			TooShortValue:         tooShortValueString,
			TooLongValue:          tooLongValueString,
		},
	}
}

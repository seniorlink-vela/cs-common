package validation

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type AppendableError interface {
	AppendErrorField(name, message string)
}

var (
	KindError       = errors.New("Incorrect kind of argument. Must be struct.")
	ValidationError = errors.New("Validation failed.")
)

type validatorFunc func(*validationRule) bool

type validationRule struct {
	ruleKey    string
	message    string
	messageKey string
	label      string
	value      reflect.Value
	params     interface{}
	validator  validatorFunc
}

var validationRuleMap = map[string]validationRule{
	"required": validationRule{
		ruleKey:   "required",
		message:   requiredMessage,
		validator: requiredValuePresent,
	},
	"email": validationRule{
		ruleKey:   "email",
		message:   emailMessage,
		validator: isEmailValid,
	},
	"min-length": validationRule{
		ruleKey:   "min-length",
		message:   tooShortMessage,
		validator: isMinimumLength,
	},
	"max-length": validationRule{
		ruleKey:   "max-length",
		message:   tooLongMessage,
		validator: isBelowMaximumLength,
	},
	"values": validationRule{
		ruleKey:   "values",
		message:   validValueMessage,
		validator: isValueValid,
	},
	"values-insensitive": validationRule{
		ruleKey:   "values-insensitive",
		message:   validValueMessage,
		validator: isValueValidInsensitive,
	},
	"not-zero": validationRule{
		ruleKey:   "not-zero",
		message:   requiredMessage,
		validator: isNotZero,
	},
}

// Error messages
const (
	requiredMessage   = "This is a required field"
	emailMessage      = "This is not a valid email address"
	tooShortMessage   = "This must be at least %d characters"
	tooLongMessage    = "This must not be longer than %d characters"
	validValueMessage = "This must be one of the following values: %s"
)

func ValidateStruct(s interface{}, ae AppendableError) error {
	validStruct := true
	valS := reflect.ValueOf(s)
	if valS.Kind() != reflect.Struct {
		return KindError
	}
	typeS := valS.Type()

	for i := 0; i < typeS.NumField(); i++ {
		f := typeS.Field(i)
		fName := fieldName(f)
		validationRules := f.Tag.Get("validation")
		if validationRules != "" {
			rules := strings.Split(validationRules, ",")
			trimSliceValues(rules)
			required, j := contains(rules, "required")
			fieldVal := valS.Field(i)
			if required {
				rules = remove(rules, j)
				rule := validationRuleMap["required"]
				rule.value = fieldVal
				rule.messageKey = fName
				if !rule.validator(&rule) {
					validStruct = false
					ae.AppendErrorField(fName, rule.message)
				}
			}
			for _, rule := range rules {
				ruleType := strings.SplitN(rule, ":", 2)
				rule := validationRuleMap[ruleType[0]]
				rule.value = fieldVal
				switch rule.ruleKey {
				case "email":
					rule.messageKey = fName
				case "min-length":
					// Being lazy about checks here, it should be safe to assume
					// that we would know how to figure out why validation of
					// our models isn't behaving as expected.
					length, _ := strconv.Atoi(ruleType[1])
					rule.messageKey = fmt.Sprintf("%s_too_short", fName)
					rule.message = fmt.Sprintf(tooShortMessage, length)
					rule.params = length
				case "max-length":
					// Being lazy about checks here, it should be safe to assume
					// that we would know how to figure out why validation of
					// our models isn't behaving as expected.
					length, _ := strconv.Atoi(ruleType[1])
					rule.messageKey = fmt.Sprintf("%s_too_long", fName)
					rule.message = fmt.Sprintf(tooLongMessage, length)
					rule.params = length
				case "values":
					validValues := strings.Split(ruleType[1], "|")
					trimSliceValues(validValues)
					rule.messageKey = fName
					rule.message = fmt.Sprintf(validValueMessage, strings.Join(validValues, ", "))
					rule.params = validValues
				case "values-insensitive":
					validValues := strings.Split(ruleType[1], "|")
					trimSliceValues(validValues)
					rule.messageKey = fName
					rule.message = fmt.Sprintf(validValueMessage, strings.Join(validValues, ", "))
					rule.params = validValues
				case "not-zero":
					rule.messageKey = fName
				default:
					// If there isn't a rule we can execute on, just move on to the next field.
					continue
				}
				if !rule.validator(&rule) {
					validStruct = false
					ae.AppendErrorField(rule.messageKey, rule.message)
				}
			}
		}
	}
	if !validStruct {
		return ValidationError
	}
	return nil
}

// Basic check for required data being present.  For non-string data,
// We only check for `nil`.
func requiredValuePresent(r *validationRule) bool {
	fieldVal := r.value
	// We follow a slightly different path here, since required
	// fields may be values other than strings.
	if fieldVal.Type().Kind() == reflect.Ptr {
		if fieldVal.IsNil() {
			return false
		} else {
			t := fieldVal.Elem().Type()
			if t.Kind() == reflect.String && fieldVal.Elem().Len() == 0 {
				return false
			}
		}
	} else {
		t := fieldVal.Type()
		if t.Kind() == reflect.String && fieldVal.Len() == 0 {
			return false
		}
	}
	return true
}

// Basic validity check for email
// it is a badly formatted email if it does not have exactly 1 @,
// the last dot must be after the @, and the @ must not be the 1st character
func isEmailValid(r *validationRule) bool {
	email := getFieldValue(r.value)
	// We've already checked for required previously, so an empty
	// string should not fail here
	if strings.TrimSpace(email) == "" {
		return true
	}
	return isValidEmail(email)
}

func isValueValid(r *validationRule) bool {
	value := getFieldValue(r.value)
	allowed := r.params.([]string)
	// We've already checked for required previously, so an empty
	// string should not fail here
	if strings.TrimSpace(value) == "" {
		return true
	}
	valid, _ := contains(allowed, value)
	return valid
}

func isValueValidInsensitive(r *validationRule) bool {
	value := getFieldValue(r.value)
	value = strings.ToLower(value)
	allowed := r.params.([]string)
	lowerCaseSliceValues(allowed)
	// We've already checked for required previously, so an empty
	// string should not fail here
	if strings.TrimSpace(value) == "" {
		return true
	}
	valid, _ := contains(allowed, value)
	return valid
}

func isBelowMaximumLength(r *validationRule) bool {
	length := r.params.(int)
	value := getFieldValue(r.value)
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		// We've already checked for required, so there is no point in checking an empty string
		return true
	} else if len(value) > length {
		return false
	}
	return true
}

func isMinimumLength(r *validationRule) bool {
	length := r.params.(int)
	value := getFieldValue(r.value)
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		// We've already checked for required, so there is no point in checking an empty string
		return true
	} else if len(value) < length {
		return false
	}
	return true
}

func fieldName(f reflect.StructField) string {
	name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
	if name == "-" || name == "" {
		name = f.Name
	}
	return name
}

func getFieldValue(valueField reflect.Value) string {
	var value string

	if valueField.Type().Kind() == reflect.Ptr {
		if !valueField.IsNil() {
			value = fmt.Sprintf("%s", valueField.Elem().Interface())
		} else {
			value = ""
		}
	} else {
		value = fmt.Sprintf("%s", valueField.Interface())
	}
	return value
}

func isNotZero(r *validationRule) bool {
	v := r.value
	if v.Type().Kind() == reflect.Ptr {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return math.Float64bits(v.Float()) != 0
	case reflect.Struct:
		// Only check time.Time for now
		t, ok := v.Interface().(time.Time)
		if ok {
			return !t.IsZero()
		}
		return true
	default:
		return true
	}
	return true
}

// Searches a slice of strings for the passed value, and returns
// both the value, and it's index, so we can do extra manipulation
// after the fact.
func contains(list []string, item string) (bool, int) {
	for i, a := range list {
		if a == item {
			return true, i
		}
	}
	return false, -1
}

// This *might* be inefficient for really large slices, but the
// likelihood of having more than 3 or 4 items in this use case
// is *very* low, so we'll allow it.
func remove(s []string, i int) []string {
	return append(s[:i], s[i+1:]...)
}

func trimSliceValues(s []string) {
	for i, value := range s {
		s[i] = strings.TrimSpace(value)
	}
}

func lowerCaseSliceValues(s []string) {
	for i, value := range s {
		s[i] = strings.ToLower(value)
	}
}

// IsValidEmail provides basic validity for email
func isValidEmail(email string) bool {
	validEmailRE := "^([^@\\s]+)@([^@\\s]+)\\.([^@\\s]+)$"
	emailRE := regexp.MustCompile(validEmailRE)
	matches := emailRE.FindAllString(email, -1)
	return len(matches) > 0
}

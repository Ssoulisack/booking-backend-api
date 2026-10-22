package utils

import "reflect"

func ValidateModel(model interface{}, requiredFields []string) bool {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}

	for _, fieldName := range requiredFields {
		field := v.FieldByName(fieldName)
		if !field.IsValid() {
			return false
		}

		switch field.Kind() {
		case reflect.String:
			if field.String() == "" {
				return false
			}
		case reflect.Int, reflect.Int64:
			if field.Int() == 0 {
				return false
			}
		case reflect.Uint, reflect.Uint64:
			if field.Uint() == 0 {
				return false
			}
		case reflect.Float64:
			if field.Float() == 0 {
				return false
			}
		case reflect.Ptr, reflect.Interface:
			if field.IsNil() {
				return false
			}
		}
	}
	return true
}

package utils

import (
	"reflect"
)

func EqualMapValues(a, b map[string]interface{}) bool {
	return (len(a) == 0 && len(b) == 0) || reflect.DeepEqual(a, b)
}

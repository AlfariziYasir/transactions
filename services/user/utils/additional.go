package utils

import (
	"encoding/json"
	"reflect"
)

func StructToMap(data any) (map[string]any, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var res map[string]any
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func GetColumn(obj interface{}) []string {
	result := make([]string, 0)
	v := reflect.ValueOf(obj)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag != "" {
			result = append(result, tag)
		}
	}

	return result
}

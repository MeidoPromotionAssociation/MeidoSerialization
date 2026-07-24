// Package strictjson 提供按 Go 类型表达线格式可空性的 editing JSON 共用严格解码器
// Package strictjson provides the shared strict decoder for editing JSON whose Go types model wire nullability
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

var rawMessageType = reflect.TypeOf(json.RawMessage{})

// Decode 严格解码唯一 JSON 值，拒绝未知字段、尾随内容以及不能表示 null 的 Go 类型位置
// Decode strictly decodes one JSON value and rejects unknown fields, trailing content, and null at Go type positions that cannot represent it
func Decode(data []byte, out any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("trailing content: %w", err)
	}

	targetType := reflect.TypeOf(out)
	if targetType == nil || targetType.Kind() != reflect.Pointer {
		return nil
	}
	return rejectInvalidNull(data, targetType.Elem(), "$")
}

// rejectInvalidNull 按目标 Go 类型递归拒绝值类型位置中的显式 JSON null
// rejectInvalidNull recursively rejects explicit JSON null at value-typed positions according to the target Go type
func rejectInvalidNull(data []byte, targetType reflect.Type, path string) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		if typeCanRepresentNull(targetType) {
			return nil
		}
		return fmt.Errorf("%s must not be null", path)
	}

	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType == rawMessageType || targetType.Kind() == reflect.Interface {
		return nil
	}

	switch targetType.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &fields); err != nil {
			return nil
		}
		for name, raw := range fields {
			fieldType, ok := findJSONFieldType(targetType, name)
			if !ok {
				continue
			}
			if err := rejectInvalidNull(raw, fieldType, path+"."+name); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		var elements []json.RawMessage
		if err := json.Unmarshal(trimmed, &elements); err != nil {
			return nil
		}
		for index, raw := range elements {
			if err := rejectInvalidNull(raw, targetType.Elem(), fmt.Sprintf("%s[%d]", path, int64(index))); err != nil {
				return err
			}
		}
	case reflect.Map:
		var values map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return nil
		}
		for key, raw := range values {
			if err := rejectInvalidNull(raw, targetType.Elem(), path+"["+strconv.Quote(key)+"]"); err != nil {
				return err
			}
		}
	}
	return nil
}

// findJSONFieldType 按 encoding/json 的字段名规则查找结构体字段类型
// findJSONFieldType finds a struct field type using the field-name rules of encoding/json
func findJSONFieldType(targetType reflect.Type, name string) (reflect.Type, bool) {
	var folded reflect.Type
	for _, field := range reflect.VisibleFields(targetType) {
		if field.PkgPath != "" {
			continue
		}
		tagName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tagName == "-" {
			continue
		}
		fieldBaseType := field.Type
		for fieldBaseType.Kind() == reflect.Pointer {
			fieldBaseType = fieldBaseType.Elem()
		}
		if field.Anonymous && tagName == "" && fieldBaseType.Kind() == reflect.Struct {
			continue
		}
		jsonName := tagName
		if jsonName == "" {
			jsonName = field.Name
		}
		if jsonName == name {
			return field.Type, true
		}
		if folded == nil && strings.EqualFold(jsonName, name) {
			folded = field.Type
		}
	}
	return folded, folded != nil
}

// typeCanRepresentNull 判断目标 Go 类型是否直接表达 typed null
// typeCanRepresentNull reports whether the target Go type directly represents typed null
func typeCanRepresentNull(targetType reflect.Type) bool {
	switch targetType.Kind() {
	case reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}

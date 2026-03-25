package blackstart

import (
	"fmt"
	"reflect"
)

// InputAs converts an Input value into the requested type T.
// When required is true, nil/empty values for common scalar/list types are rejected.
func InputAs[T any](input Input, required bool) (T, error) {
	var zero T
	if input == nil {
		if required {
			return zero, fmt.Errorf("required input is missing")
		}
		return zero, nil
	}

	if !input.IsStatic() {
		return zero, fmt.Errorf("input value is not static")
	}

	val := input.Any()
	if val == nil {
		if required {
			return zero, fmt.Errorf("value cannot be nil")
		}
		return zero, nil
	}

	out, err := coerceValue[T](val)
	if err != nil {
		return zero, err
	}

	if required {
		if err = validateRequiredValue(out); err != nil {
			return zero, err
		}
	}

	return out, nil
}

// ContextInputAs reads input `key` from a ModuleContext and converts it to type T.
func ContextInputAs[T any](ctx ModuleContext, key string, required bool) (T, error) {
	var zero T
	input, err := ctx.Input(key)
	if err != nil {
		if required {
			return zero, fmt.Errorf("missing required input %s: %w", key, err)
		}
		return zero, nil
	}

	out, err := InputAs[T](input, required)
	if err != nil {
		return zero, fmt.Errorf("invalid input %s: %w", key, err)
	}
	return out, nil
}

// coerceValue converts a raw input value into T using assignment/conversion rules and
// special handling for common YAML/JSON decode shapes.
func coerceValue[T any](value any) (T, error) {
	var zero T
	targetType := reflect.TypeFor[T]()

	// Special case: allow bool to satisfy *bool inputs.
	if targetType.Kind() == reflect.Ptr && targetType.Elem().Kind() == reflect.Bool {
		if b, ok := value.(bool); ok {
			ptr := &b
			return any(ptr).(T), nil
		}
	}

	// Special case: YAML/JSON lists commonly decode to []any.
	if targetType.Kind() == reflect.Slice {
		elemType := targetType.Elem()
		switch x := value.(type) {
		case []any:
			out := reflect.MakeSlice(targetType, 0, len(x))
			for i, item := range x {
				if item == nil {
					return zero, fmt.Errorf("value[%d] must not be nil", i)
				}
				itemValue := reflect.ValueOf(item)
				itemType := itemValue.Type()

				if elemType.Kind() == reflect.String {
					if itemType.Kind() != reflect.String {
						return zero, fmt.Errorf("value[%d] must be string, got %T", i, item)
					}
				}

				if itemType.AssignableTo(elemType) {
					out = reflect.Append(out, itemValue)
					continue
				}
				if itemType.ConvertibleTo(elemType) && elemType.Kind() != reflect.String {
					out = reflect.Append(out, itemValue.Convert(elemType))
					continue
				}
				return zero, fmt.Errorf("value[%d] must be %v, got %T", i, elemType, item)
			}
			return out.Interface().(T), nil
		case string:
			if elemType.Kind() == reflect.String {
				out := reflect.MakeSlice(targetType, 1, 1)
				out.Index(0).Set(reflect.ValueOf(x).Convert(elemType))
				return out.Interface().(T), nil
			}
		}
	}

	if value == nil {
		return zero, fmt.Errorf("value is nil")
	}

	v := reflect.ValueOf(value)
	if v.Type().AssignableTo(targetType) {
		return v.Interface().(T), nil
	}

	if v.Type().ConvertibleTo(targetType) {
		cv := v.Convert(targetType)
		return cv.Interface().(T), nil
	}

	return zero, fmt.Errorf("expected %v, got %T", targetType, value)
}

// validateRequiredValue enforces non-empty/non-nil semantics for required input values.
func validateRequiredValue[T any](value T) error {
	v := any(value)
	switch x := v.(type) {
	case string:
		if x == "" {
			return fmt.Errorf("value cannot be empty")
		}
	case []string:
		if len(x) == 0 {
			return fmt.Errorf("value cannot be empty")
		}
		for i, s := range x {
			if s == "" {
				return fmt.Errorf("value[%d] cannot be empty", i)
			}
		}
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && rv.Kind() == reflect.Ptr && rv.IsNil() {
			return fmt.Errorf("value cannot be nil")
		}
	}
	return nil
}

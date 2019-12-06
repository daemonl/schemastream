package schemastream

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/pkg/errors"
)

type Decoder struct {
	*json.Decoder
	schema *spec.Schema
	into   reflect.Value
	path   []string
}

func ValidateParse(reader io.Reader, into interface{}, schema *spec.Schema) error {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	valueOf := reflect.ValueOf(into).Elem()
	return decodeAnything(Decoder{
		Decoder: decoder,
		schema:  schema,
		into:    valueOf,
		path:    []string{},
	})
}

func printToken(prefix string, token json.Token) {
	fmt.Printf("Token %s: %T %v\n", prefix, token, token)
}

func decodeAnything(decoder Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}

	printToken(strings.Join(decoder.path, ","), token)

	var intoKind reflect.Kind

	if decoder.into.IsValid() {
		intoKind = decoder.into.Type().Kind()
	}

	if tokenVal, ok := token.(json.Delim); ok {
		switch tokenVal {
		case '{':
			return decodeObject(decoder)
		default:
			return fmt.Errorf("Unknown Token %s", tokenVal)
		}
	}

	if decoder.schema == nil {
		return nil
	}

	fmt.Printf("Decode %v into %v -> %s\n", token, intoKind.String(), decoder.schema.Type)

	switch tokenVal := token.(type) {
	case string:
		if !decoder.schema.Type.Contains("string") {
			return fmt.Errorf("Unable to cast string to %s", decoder.schema.Type)
		}
		if intoKind != reflect.String {
			return fmt.Errorf("Schema type string is not compatible with variable type %s", intoKind.String())
		}
		decoder.into.Set(reflect.ValueOf(tokenVal))

	case json.Number:
		if !(decoder.schema.Type.Contains("number") || decoder.schema.Type.Contains("integer")) {
			return fmt.Errorf("Unexpected number")
		}

		if intoKind == reflect.Float64 || intoKind == reflect.Float32 {
			floatVal, err := tokenVal.Float64()
			if err != nil {
				return err
			}

			if decoder.schema.Maximum != nil {
				if floatVal > *decoder.schema.Maximum {
					return fmt.Errorf("Exceeds Maximum")
				}
			}

			decoder.into.Set(reflect.ValueOf(floatVal))
		} else if intoKind >= reflect.Int && intoKind <= reflect.Uint64 {
			intVal, err := tokenVal.Int64()
			if err != nil {
				return err
			}

			if decoder.schema.Maximum != nil {
				if float64(intVal) > *decoder.schema.Maximum {
					return fmt.Errorf("Exceeds Maximum")
				}
			}

			decoder.into.Set(reflect.ValueOf(intVal))
		} else {
			return fmt.Errorf("Unable to cast json number to %s", intoKind.String())
		}

	case bool:
		if decoder.schema.Type.Contains("bool") {
			decoder.into.Set(reflect.ValueOf(tokenVal))
		} else {
			return fmt.Errorf("Unable to cast bool to %s", decoder.schema.Type)
		}

	case nil:
	default:
		return fmt.Errorf("Unknown Type: %T", token)
	}

	return nil
}

func decodeObject(decoder Decoder) error {
	fmt.Printf("Decode Object into %s\n", decoder.into.Type().Name())
	if !decoder.schema.Type.Contains("object") {
		return fmt.Errorf("Not expecting an object")
	}

	fieldsByTag := map[string]reflect.Value{}

	for idx := 0; idx < decoder.into.NumField(); idx++ {
		field := decoder.into.Field(idx)
		jsonTag, ok := decoder.into.Type().Field(idx).Tag.Lookup("json")
		if ok {
			tagBase := strings.Split(jsonTag, ",")[0]
			fieldsByTag[tagBase] = field
		}
	}

	for {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		if keyToken == json.Delim('}') {
			return nil
		}
		keyName, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("Expected a string got %v", keyToken)
		}

		fmt.Printf("Set %s\n", keyName)

		field, _ := fieldsByTag[keyName]
		fieldSchemaVal, ok := decoder.schema.Properties[keyName]
		fieldSchema := &fieldSchemaVal
		if !ok {
			if decoder.schema.AdditionalProperties == nil || !decoder.schema.AdditionalProperties.Allows {
				return fmt.Errorf("Unknown Property %s", keyName)
			}
			fieldSchema = nil
		}

		fieldPath := append(decoder.path, keyName)
		if err := decodeAnything(Decoder{
			Decoder: decoder.Decoder,
			into:    field,
			schema:  fieldSchema,
			path:    fieldPath,
		}); err != nil {
			return errors.Wrapf(err, "At path %s", strings.Join(fieldPath, "."))
		}

	}

}

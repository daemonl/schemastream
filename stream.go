package schemastream

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/pkg/errors"
)

type decoderRes struct {
	token json.Token
	err   error
}

type Decoder struct {
	jsonDecoder *json.Decoder
	nextRes     *decoderRes
}

func (d *Decoder) Token() (json.Token, error) {
	if d.nextRes != nil {
		res := d.nextRes
		d.nextRes = nil
		return res.token, res.err
	}
	return d.jsonDecoder.Token()
}

func (d *Decoder) nextToken() (json.Token, error) {
	if d.nextRes == nil {
		token, err := d.jsonDecoder.Token()
		d.nextRes = &decoderRes{
			token: token,
			err:   err,
		}
	}
	return d.nextRes.token, d.nextRes.err
}

type Baton struct {
	schema *spec.Schema
	into   reflect.Value
	path   []string
}

func ValidateParse(reader io.Reader, into interface{}, schema *spec.Schema) error {
	jsonDecoder := json.NewDecoder(reader)
	jsonDecoder.UseNumber()
	valueOf := reflect.ValueOf(into).Elem()
	decoder := &Decoder{
		jsonDecoder: jsonDecoder,
	}
	baton := Baton{
		schema: schema,
		into:   valueOf,
		path:   []string{},
	}
	return decodeAnything(decoder, baton)

}

func printToken(prefix string, token json.Token) {
	fmt.Printf("Token %s: %T %v\n", prefix, token, token)
}

func decodeAnything(decoder *Decoder, baton Baton) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}

	printToken(strings.Join(baton.path, ","), token)

	if tokenVal, ok := token.(json.Delim); ok {
		switch tokenVal {
		case '{':
			return decodeObject(decoder, baton)
		case '[':
			return decodeArray(decoder, baton)
		default:
			return fmt.Errorf("Unknown Token %s", tokenVal)
		}
	}

	if baton.schema == nil {
		return nil
	}

	if !baton.into.IsValid() {
		return nil
	}

	var intoKind reflect.Kind
	into := baton.into
	fmt.Printf("Item Type for Elem: %s\n", into.String())

	if into.IsValid() {
		if into.Kind() == reflect.Ptr {
			into = into.Elem()
		}
		fmt.Printf("Converted Item Type for Elem: %s\n", into.String())
		intoKind = into.Type().Kind()
	}

	fmt.Printf("Decode %v into %v -> %s\n", token, intoKind.String(), baton.schema.Type)

	if err := validate.AgainstSchema(baton.schema, token, strfmt.Default); err != nil {
		return err
	}

	switch tokenVal := token.(type) {

	case string:
		if intoKind != reflect.String {
			return fmt.Errorf("Schema type string is not compatible with variable type %s", intoKind.String())
		}
		into.Set(reflect.ValueOf(tokenVal))

	case json.Number:
		if !(baton.schema.Type.Contains("number") || baton.schema.Type.Contains("integer")) {
			return fmt.Errorf("Unexpected number")
		}

		if intoKind == reflect.Float64 || intoKind == reflect.Float32 {
			floatVal, err := tokenVal.Float64()
			if err != nil {
				return err
			}

			into.Set(reflect.ValueOf(floatVal))
		} else if intoKind >= reflect.Int && intoKind <= reflect.Uint64 {
			intVal, err := tokenVal.Int64()
			if err != nil {
				return err
			}

			into.Set(reflect.ValueOf(intVal))
		} else {
			return fmt.Errorf("Unable to cast json number to %s", intoKind.String())
		}

	case bool:
		if !baton.schema.Type.Contains("boolean") {
			return fmt.Errorf("Unable to cast bool to %s", baton.schema.Type)
		}

		if intoKind != reflect.Bool {
			return fmt.Errorf("Unable to cast json bool to %s", intoKind.String())
		}

		into.Set(reflect.ValueOf(tokenVal))

	case nil:
	default:
		return fmt.Errorf("Unknown Type: %T", token)
	}

	return nil
}

func decodeArray(decoder *Decoder, baton Baton) error {
	if !baton.schema.Type.Contains("array") {
		return fmt.Errorf("Not expecting an array")
	}

	itemType := baton.into.Type().Elem()
	fmt.Printf("Item Type for Array: %s\n", itemType.String())

	arrayValue := baton.into
	idx := 0
	for {
		// Don't consume the token
		keyToken, err := decoder.nextToken()
		if err != nil {
			return err
		}
		if keyToken == json.Delim(']') {
			baton.into.Set(arrayValue)
			// discard Next
			decoder.Token()
			return nil
		}

		field := reflect.New(itemType)
		fieldSchema := baton.schema.Items.Schema
		fieldPath := append(baton.path, fmt.Sprintf("%d", idx))

		if err := decodeAnything(decoder, Baton{
			into:   field,
			schema: fieldSchema,
			path:   fieldPath,
		}); err != nil {
			return errors.Wrapf(err, "At path %s", strings.Join(fieldPath, "."))
		}

		arrayValue = reflect.Append(arrayValue, field.Elem())
		idx++
	}

}

func decodeObject(decoder *Decoder, baton Baton) error {
	fmt.Printf("Decode Object into %s\n", baton.into.Type().Name())
	if !baton.schema.Type.Contains("object") {
		return fmt.Errorf("Not expecting an object")
	}

	fieldsByTag := map[string]reflect.Value{}
	backupFieldsByTag := map[string]reflect.Value{}

	for idx := 0; idx < baton.into.NumField(); idx++ {
		field := baton.into.Field(idx)
		fieldType := baton.into.Type().Field(idx)
		jsonTag, ok := fieldType.Tag.Lookup("json")
		if ok {
			tagBase := strings.Split(jsonTag, ",")[0]
			fieldsByTag[tagBase] = field
		} else {
			backupFieldsByTag[strings.ToLower(fieldType.Name)] = field
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

		field, ok := fieldsByTag[keyName]
		if !ok {
			field, ok = backupFieldsByTag[strings.ToLower(keyName)]
			if !ok {
				// TODO: Skip?
			}
		}
		fieldSchemaVal, ok := baton.schema.Properties[keyName]
		fieldSchema := &fieldSchemaVal
		if !ok {
			if baton.schema.AdditionalProperties == nil || !baton.schema.AdditionalProperties.Allows {
				return fmt.Errorf("Unknown Property %s", keyName)
			}
			fieldSchema = nil
		}

		fieldPath := append(baton.path, keyName)
		if err := decodeAnything(decoder, Baton{
			into:   field,
			schema: fieldSchema,
			path:   fieldPath,
		}); err != nil {
			return errors.Wrapf(err, "At path %s", strings.Join(fieldPath, "."))
		}

	}

}

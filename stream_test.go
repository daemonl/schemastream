package schemastream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-openapi/spec"
)

func buildSchema(data string) *spec.Schema {

	var schema = (&spec.Schema{})

	if err := json.Unmarshal([]byte(data), schema); err != nil {
		panic(err.Error())
	}
	return schema

}

var defaultSchema = `{
	"type": "object",
	"properties": {
		"string": { "type": "string" },
		"float64": { "type": "number", "maximum": 10 },
		"unTagged": { "type": "boolean" },
		"child": { 
			"type": "object",
			"properties": {
				"foo": { "type": "string" }
			}
		},
		"arr": {
			"type": "array",
			"items": {
				"type": "string"
			}
		}
	}
}`

type TestStruct struct {
	String  string  `json:"string"`
	Float64 float64 `json:"float64"`

	UnTagged bool

	Extra string `json:"extra"` // Not in the Schema

	Child struct {
		Foo string `json:"foo"`
	} `json:"child"`

	Arr []string `json:"arr"`
}

func TestNonPointerError(t *testing.T) {
	notPointer := TestStruct{}
	err := ValidateParse(strings.NewReader(`{}`), notPointer, buildSchema(defaultSchema))
	if err == nil {
		t.Errorf("Expected Error")
	}
	_, ok := err.(*json.InvalidUnmarshalError)
	if !ok {
		t.Errorf("Wrong error: %T", err)
	}

}

func TestParseEmpty(t *testing.T) {
	into := TestStruct{}
	if err := ValidateParse(strings.NewReader(`{}`), &into, buildSchema(defaultSchema)); err != nil {
		t.Fatal(err.Error())
	}
}

func TestParseData(t *testing.T) {
	into := TestStruct{}

	if err := ValidateParse(strings.NewReader(`
		{
			"string": "fooVal",
			"float64": 1,
			"unTagged": true,
			"child": {"foo": "bar"}
		}
	`), &into, buildSchema(defaultSchema)); err != nil {
		t.Fatal(err.Error())
	}

	if into.String != "fooVal" {
		t.Errorf("at foo, got %s", into.String)
	}
	if into.Float64 != 1.0 {
		t.Errorf("at bar, got %f", into.Float64)
	}

	if into.UnTagged != true {
		t.Errorf("UnTagged was not set")
	}

	if into.Child.Foo != "bar" {
		t.Errorf("At child.foo, got %s", into.Child.Foo)
	}
}

func TestParsePointers(t *testing.T) {
	into := struct {
		Omit      *string `json:"omit"`
		NullValue *string `json:"nullValue"`
		SetValue  *string `json:"setValue"`

		Child *struct {
			Foo string `json:"foo"`
		} `json:"child"`
	}{}

	if err := ValidateParse(strings.NewReader(`
		{
			"nullValue": null,
			"setValue": "set",
			"child": {"foo": "bar"}
		}
	`), &into, buildSchema(`{
	"type": "object",
	"properties": {
		"nullValue": { "type": "string" },
		"setValue": { "type": "string" },
		"omit": { "type": "string" },
		"child": { 
			"type": "object",
			"properties": {
				"foo": { "type": "string" }
			}
		}
	}

	}`)); err != nil {
		t.Fatal(err.Error())
	}

	if into.Omit != nil {
		t.Errorf("at Omit, got %v", into.Omit)
	}
	if into.NullValue != nil {
		t.Errorf("at Omit, got %v", into.NullValue)
	}
	if into.SetValue == nil || *into.SetValue != "set" {
		t.Errorf("at float64, got %v", into.SetValue)
	}

	if into.Child.Foo != "bar" {
		t.Errorf("At child.foo, got %s", into.Child.Foo)
	}

}

func TestSkipMissingAllowed(t *testing.T) {
	into := TestStruct{}
	schema := buildSchema(defaultSchema)
	schema.AdditionalProperties = &spec.SchemaOrBool{Allows: true}
	if err := ValidateParse(strings.NewReader(`{"missing":"foo"}`), &into, schema); err != nil {
		t.Fatal(err.Error())
	}
}

func TestSkipMissingNotAllowed(t *testing.T) {
	into := TestStruct{}
	err := ValidateParse(strings.NewReader(`{"missing":"foo"}`), &into, buildSchema(defaultSchema))
	if err == nil {
		t.Fatal("Expected an error")
	}
}

func TestSkipExtra(t *testing.T) {
	into := TestStruct{}
	schema := buildSchema(defaultSchema)
	schema.AdditionalProperties = &spec.SchemaOrBool{Allows: true}
	if err := ValidateParse(strings.NewReader(`{"extra":"foo"}`), &into, schema); err != nil {
		t.Fatal(err.Error())
	}
	if into.Extra != "" {
		t.Errorf("extra was set, it should be ignored when missing from the schema")
	}

}

func TestInvalidData(t *testing.T) {
	into := TestStruct{}

	err := ValidateParse(strings.NewReader(`{"string":"fooVal","float64":101}`), &into, buildSchema(defaultSchema))
	if err == nil {
		t.Fatal("Did not throw")
	}
	msg := err.Error()
	if !strings.Contains(msg, "float64") {
		t.Fatalf("Path name missing from error: %s", msg)
	}
	t.Log(msg)

}

func TestArray(t *testing.T) {
	into := struct {
		Arr []string
	}{}

	err := ValidateParse(strings.NewReader(`
	{
		"arr": ["a", "b"]
	}
	`), &into, buildSchema(`{
		"type": "object",
		"properties": {
			"arr": {
				"type": "array",
				"items": {
					"type": "string"
				}
			}
		}
	}`))

	if err != nil {
		t.Fatal(err.Error())
	}

	if len(into.Arr) != 2 {
		t.Fatalf("Got %d items, %v", len(into.Arr), into.Arr)
	}
	if into.Arr[0] != "a" || into.Arr[1] != "b" {
		t.Errorf("Wrong Values: %v", into.Arr)
	}

}

func TestArrayPointer(t *testing.T) {
	into := struct {
		Arr []*string
	}{}

	err := ValidateParse(strings.NewReader(`
	{
		"arr": ["a", null, "b"]
	}
	`), &into, buildSchema(`{
		"type": "object",
		"properties": {
			"arr": {
				"type": "array",
				"items": {
					"type": "string"
				}
			}
		}
	}`))

	if err != nil {
		t.Fatal(err.Error())
	}

	if len(into.Arr) != 3 {
		t.Fatalf("Got %d items, %v", len(into.Arr), into.Arr)
	}

	if into.Arr[0] == nil || *into.Arr[0] != "a" {
		t.Errorf("Wrong Values: %v", into.Arr)
	}
	if into.Arr[2] == nil || *into.Arr[2] != "b" {
		t.Errorf("Wrong Values: %v", into.Arr)
	}
	if into.Arr[1] != nil {
		t.Errorf("Should be null at 1: %v", into.Arr)
	}

}

func TestPointerToArray(t *testing.T) {
	into := struct {
		Arr    *[]string
		NilArr *[]string
	}{}

	err := ValidateParse(strings.NewReader(`
	{
		"arr": ["a", "b"],
		"nilArr": null
	}
	`), &into, buildSchema(`{
		"type": "object",
		"properties": {
			"arr": {
				"type": "array",
				"items": {
					"type": "string"
				}
			},
			"nilArr": {
				"type": "array",
				"items": {
					"type": "string"
				}
			}
		}
	}`))

	if err != nil {
		t.Fatal(err.Error())
	}

	if into.Arr == nil {
		t.Fatalf("Got Nil")
	} else {
		arr := *into.Arr
		if len(arr) != 2 {
			t.Fatalf("Got %d items, %v", len(arr), arr)
		}
		if arr[0] != "a" || arr[1] != "b" {
			t.Errorf("Wrong Values: %v", arr)
		}
	}

}

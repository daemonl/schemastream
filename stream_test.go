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

}

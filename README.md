Schema Stream
=============

Status: Proof of Concept.

A replacement for golang's json Decoder which also validates against json
schema objects.

Why:

Existing approaches either parse the object twice, or validate the resulting struct.

When using a concrete struct as the parse target, a number of validations are not possible:

- Any Additional Properties are ignored, can't throw an error
- Required values will look like they are set when they aren't (workaround: pointers)

Missing Type demo:

```go
type Target struct {
	Foo string `json:"foo"`
}

target := &Target{}

json.Decode([]byte(`{"notAllowed":"butGotSet}`), target)

schema := Schema{
	Required: []string{"foo"},
	AdditionalProperties: false,
}

Validate(target, schema)
// Is valid.
```



package codec

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestInterop_JSON_MsgPack_RoundTrip(t *testing.T) {
	// Original JSON
	sourceJSON := []byte(`{"id":123,"name":"Alice","active":true,"score":45.67}`)

	// JSON -> UIR
	decoder, _ := GetDecoder("json")
	uirNode, err := decoder.Decode(sourceJSON)
	if err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	// UIR -> MsgPack
	encoder, _ := GetEncoder("msgpack")
	msgPackBytes, err := encoder.Encode(uirNode)
	if err != nil {
		t.Fatalf("Failed to encode to MsgPack: %v", err)
	}

	// MsgPack -> UIR
	msgPackDecoder, _ := GetDecoder("msgpack")
	uirNode2, err := msgPackDecoder.Decode(msgPackBytes)
	if err != nil {
		t.Fatalf("Failed to decode MsgPack: %v", err)
	}

	// UIR -> JSON
	jsonEncoder, _ := GetEncoder("json")
	finalJSON, err := jsonEncoder.Encode(uirNode2)
	if err != nil {
		t.Fatalf("Failed to encode back to JSON: %v", err)
	}

	// Validate they represent the same data structure (maps are unordered so exact string match may fail)
	// We'll decode both to map[string]interface{} and compare using reflect.DeepEqual
	var originalMap, finalMap map[string]interface{}
	
	// Decode original JSON manually for comparison
	importJSON(sourceJSON, &originalMap)
	importJSON(finalJSON, &finalMap)

	if !reflect.DeepEqual(originalMap, finalMap) {
		t.Errorf("Round trip failed. Expected %v, got %v\nOriginal JSON: %s\nFinal JSON: %s", originalMap, finalMap, sourceJSON, finalJSON)
	}
}

func importJSON(data []byte, target *map[string]interface{}) {
	json.Unmarshal(data, target)
}

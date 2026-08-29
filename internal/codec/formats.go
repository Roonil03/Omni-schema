package codec

import "strings"

// AdvertisedFormats is the HTTP conversion matrix covered by integration tests.
var AdvertisedFormats = []string{
	"json", "msgpack", "protobuf", "graphql", "avro", "odata", "capnproto", "parquet", "hdf5",
}

// NormalizeFormat maps file extensions and aliases to advertised codec names.
func NormalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), ".")) {
	case "proto", "pb", "protobuf":
		return "protobuf"
	case "capnp", "capnproto":
		return "capnproto"
	case "gql", "graphql":
		return "graphql"
	case "h5", "hdf", "hdf5":
		return "hdf5"
	case "messagepack", "msgpck", "msgpack":
		return "msgpack"
	case "pq", "parquet":
		return "parquet"
	case "avro":
		return "avro"
	case "odata":
		return "odata"
	case "json":
		return "json"
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func RequiresExternalSchema(format string) bool {
	switch NormalizeFormat(format) {
	case "protobuf", "capnproto":
		return true
	default:
		return false
	}
}

func IsBinaryFormat(format string) bool {
	switch NormalizeFormat(format) {
	case "protobuf", "msgpack", "capnproto", "parquet", "hdf5", "avro":
		return true
	default:
		return false
	}
}

func IsContainerFormat(format string) bool {
	switch NormalizeFormat(format) {
	case "parquet", "hdf5", "avro":
		return true
	default:
		return false
	}
}

package codec

// AdvertisedFormats is the HTTP conversion matrix covered by integration tests.
var AdvertisedFormats = []string{
	"json", "msgpack", "protobuf", "graphql", "avro", "odata", "capnproto", "parquet", "hdf5",
}

func RequiresExternalSchema(format string) bool {
	switch format {
	case "protobuf", "capnproto":
		return true
	default:
		return false
	}
}

func IsBinaryFormat(format string) bool {
	switch format {
	case "protobuf", "msgpack", "capnproto", "parquet", "hdf5", "avro":
		return true
	default:
		return false
	}
}

func IsContainerFormat(format string) bool {
	switch format {
	case "parquet", "hdf5", "avro":
		return true
	default:
		return false
	}
}

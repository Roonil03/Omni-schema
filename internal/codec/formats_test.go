package codec

import "testing"

func TestNormalizeFormat(t *testing.T) {
	cases := map[string]string{
		"proto":       "protobuf",
		".pb":         "protobuf",
		"capnp":       "capnproto",
		"gql":         "graphql",
		"h5":          "hdf5",
		"messagepack": "msgpack",
		"JSON":        "json",
	}
	for in, want := range cases {
		if got := NormalizeFormat(in); got != want {
			t.Errorf("NormalizeFormat(%q)=%q want %q", in, got, want)
		}
	}
}

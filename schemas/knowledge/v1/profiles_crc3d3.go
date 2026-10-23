package knowledgev1

// crc3d3Profiles 构建 CRC3D3 MessagePack 表格的源码审核指南
// crc3d3Profiles builds source-reviewed guides for CRC3D3 MessagePack tables
func crc3d3Profiles() map[string]Guide {
	csvSource := source("CRC3D3 demo", "CRC3D3 demo/Assembly-CSharp/CsvManager.cs", "CsvManager.GetCsvBinary/CreateCsvIdManager", 85, 178, "CsvManager loads .neimp resources as TextAsset bytes and hands them to MessagePackSerializer.Deserialize with CsvInstances.DefaultOptions, while .nei remains the legacy csvLegacyExtensio path parsed as a CSV table.")
	optionSource := source("CRC3D3 demo", "CRC3D3 demo/WfSystem.Serialization/WfSystem/Serialization/MessagePackOption.cs", "MessagePackOption.PrivateEnumStringLz4BlockArray", 6, 22, "The CSV tables use DynamicEnumAsStringResolver plus StandardResolverAllowPrivate plus MessagePackCompression.Lz4BlockArray, so enums travel as strings and payloads may be Lz4BlockArray framed.")
	csvDataSource := source("CRC3D3 demo", "CRC3D3 demo/Assembly-CSharp/CRC/DataBase/CsvData.cs", "CsvData<T>", 5, 12, "CsvData stores its rows in a Dictionary<int, T> under an integer key, so a table row map is keyed by the row id rather than by a property name.")
	containerSource := implementationSource("CRC3D3 demo", "serialization/CRC3D3/neimp.go", "DecodeNeimp/EncodeNeimp", 26, 120, "The serializer reads the bare MessagePack root value when the payload is short and the Lz4BlockArray framing otherwise, and writes the same choice back using the game's 64-byte CompressionMinLength default.")
	valueSource := implementationSource("CRC3D3 demo", "serialization/KCES/msgpack/value.go", "DecodeValueJSON/EncodeValueJSON", 500, 700, "The shared value-tree codec writes the editing JSON as a plain dump of the MessagePack tree with no envelope, restores canonical decimal object keys as integer keys, and keeps float32 values at single precision.")
	field := fieldFrom(csvSource, optionSource, valueSource)

	neimp := guide(
		"CRC3D3 .neimp guide",
		"A MessagePack table resource that replaces the legacy .nei CSV table in CRC3D3. The editing JSON is the MessagePack value tree itself, so the document shape follows the CsvData type the file belongs to.",
		FormatVerificationSerializationVerified,
		"The CRC3D3 CSV loader, the serialization options it passes, and the shared value-tree codec were reviewed. This profile claims the container and value-tree contract only: the field layout of each CsvData type is not published as a schema, so the value tree is edited as data rather than through named members.",
		[]Source{csvSource, optionSource, csvDataSource, containerSource, valueSource},
		[]Field{
			field("/", "Normalized .neimp value tree", "The whole MessagePack value tree, which is the editing JSON root itself.", "CsvManager supplies the deserialized object graph to the CsvData type that requested the file; the container and the value tree carry no field names of their own.", "opaque_domain_data", "Keep every value at its original type: integers stay integers, floats keep their single precision, and only members documented by the specific CsvData type should change.", "critical"),
		},
	)
	neimp.FieldPatterns = []FieldPattern{
		pattern("/*", "Table value", "One node of the value tree: a scalar, an array, or a map that a specific CsvData type consumes.", "The requesting CsvData type, not CsvManager, gives this node its meaning; the game reads it by position inside a serialized object.", "schema_dependent", "Require the CsvData definition for the exact file before changing a node, and never reorder the elements of an array that the game reads by position.", csvDataSource),
		pattern("/*/{integer-key}", "Row map entry", "A row id key mapped to the serialized CsvData row.", "CsvData looks rows up by the integer id stored in its Dictionary<int, T> key.", "runtime_collection", "Keep the key equal to the row's own id field and write it as a plain decimal integer.", csvDataSource),
	}
	neimp.Rules = []Rule{
		{ID: "neimp-value-types", AppliesTo: []string{"/*"}, Severity: "error", Summary: "The editing JSON has to keep the MessagePack value types.", Details: "The game deserializes the tree into typed C# members, so a number written as a string or an integer written where a float is expected changes the wire type. Integral floats must keep a decimal point so they stay floating point, and a map key spelled as a canonical decimal integer is written back as an integer key.", Evidence: []Source{valueSource, csvDataSource}},
		{ID: "neimp-no-envelope", AppliesTo: []string{"/*"}, Severity: "error", Summary: "The JSON root is the value tree itself.", Details: "This format declares no metadata of its own: there is no version marker, format marker, or wrapper object around the tree. Adding one changes the root type and the game then reads a map where it expects an array.", Evidence: []Source{containerSource, valueSource}},
	}
	neimp.Invariants = []string{
		"A .neimp payload is either a bare MessagePack root value or Lz4BlockArray framing around one, and the writer chooses by the game's 64-byte threshold.",
		"The value tree holds only null, booleans, non-negative integers, float32, strings, arrays, and maps.",
		"The editing JSON root is the value tree itself with no envelope.",
	}

	return map[string]Guide{
		"crc3d3.neimp": neimp,
	}
}

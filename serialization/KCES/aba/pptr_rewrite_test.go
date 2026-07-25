package aba

import "testing"

func TestRewritePPtrReferences(t *testing.T) {
	pptr := func(name string, fileID int64, pathID int64) *TypeTreeValue {
		return &TypeTreeValue{
			TypeName: "PPtr<Texture2D>",
			Name:     name,
			Children: []*TypeTreeValue{
				{TypeName: "int", Name: "m_FileID", Value: fileID},
				{TypeName: "SInt64", Name: "m_PathID", Value: pathID},
			},
		}
	}
	root := &TypeTreeValue{
		TypeName: "Root",
		Children: []*TypeTreeValue{
			pptr("local", 0, 7),
			pptr("external", 2, -9),
			pptr("null", 3, 0),
		},
	}

	count, err := RewritePPtrReferences(root, func(fileID int32, pathID int64) (int32, int64, error) {
		return 0, pathID * 10, nil
	})
	if err != nil {
		t.Fatalf("RewritePPtrReferences: %v", err)
	}
	if count != 2 {
		t.Fatalf("rewritten count = %d, want 2", count)
	}

	assertPPtr := func(value *TypeTreeValue, wantFileID int64, wantPathID int64) {
		t.Helper()
		fileID, ok := value.Field("m_FileID").Int64()
		if !ok || fileID != wantFileID {
			t.Fatalf("%s fileID = %d, ok=%v, want %d", value.Name, fileID, ok, wantFileID)
		}
		pathID, ok := value.Field("m_PathID").Int64()
		if !ok || pathID != wantPathID {
			t.Fatalf("%s pathID = %d, ok=%v, want %d", value.Name, pathID, ok, wantPathID)
		}
	}
	assertPPtr(root.Children[0], 0, 70)
	assertPPtr(root.Children[1], 0, -90)
	assertPPtr(root.Children[2], 0, 0)
}

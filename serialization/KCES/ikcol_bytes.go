package KCES

// .ikcol.bytes
// Unity Resources 中使用复合扩展名保存的 IK 碰撞体文件，wire 布局与 .ikcol 完全相同
// 识别时必须匹配完整复合扩展名，不能退化为普通 .bytes 文件
// .ikcol.bytes
// IK collider file stored under a compound extension in Unity Resources, with the same wire layout as .ikcol
// Detection must match the complete compound extension instead of treating it as an ordinary .bytes file

const KCESIKColBytesExtension = ".ikcol.bytes"

var ikcolBytesPayloadDescriptor = kcesPayloadDescriptor{
	Extension:      KCESIKColBytesExtension,
	Kind:           PayloadKindIKCollider,
	LengthPrefixed: true,
}

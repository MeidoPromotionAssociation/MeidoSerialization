package aba

// .asset_scene
// KCES 场景资源包使用的扩展名，内容仍是标准 UnityFS AssetBundle，与 .aba 共用完整读写实现
// 扩展名只表达资源用途，不改变 UnityFS 头、块目录或 SerializedFile 布局
// .asset_scene
// Extension used by KCES scene resource bundles; the content remains a standard UnityFS AssetBundle and shares the complete .aba codec
// The suffix expresses resource purpose only and does not change the UnityFS header, block directory, or SerializedFile layout

const AssetSceneExtension = ".asset_scene"

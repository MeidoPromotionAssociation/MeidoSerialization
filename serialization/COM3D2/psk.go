package COM3D2

import (
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/serialization/binaryio/stream"
)

// CM3D21_PSK
// 裙子专用物理信息文件
//
// 版本 217 以上
// 新增 PanierRadiusDistribGroup
// 游戏源码只按数值 217 判断，没有说明该值与发行版本号的对应关系
// CM3D21_PSK
// Skirt-specific physics information file
//
// Version 217 and later
// PanierRadiusDistribGroup was added
// The game source only compares against the numeric value 217 and does not document how it maps to a release version

// Psk 整体描述一个 .psk 文件的结构
// Psk describes the full structure of a .psk file
type Psk struct {
	Signature                    string              `json:"Signature"`                    // 文件签名，通常为 CM3D21_PSK / File signature, usually CM3D21_PSK
	Version                      int32               `json:"Version"`                      // 文件版本号，可能随游戏更新变化但结构不一定变化 / File version, may change with game updates without structural changes
	PanierRadius                 float32             `json:"PanierRadius"`                 // 裙撑半径 / Panier radius
	PanierRadiusDistrib          AnimationCurve      `json:"PanierRadiusDistrib"`          // 裙撑半径分布曲线 / Panier radius distribution curve
	PanierRadiusDistribGroups    []PanierRadiusGroup `json:"PanierRadiusDistribGroups"`    // 裙撑半径分布组 / Panier radius distribution groups
	PanierForce                  float32             `json:"PanierForce"`                  // 裙撑力度 / Panier force
	PanierForceDistrib           AnimationCurve      `json:"PanierForceDistrib"`           // 裙撑力度分布曲线 / Panier force distribution curve
	PanierStressForce            float32             `json:"PanierStressForce"`            // 裙撑应力 / Panier stress force
	StressDegreeMin              float32             `json:"StressDegreeMin"`              // 最小应力度 / Minimum stress degree
	StressDegreeMax              float32             `json:"StressDegreeMax"`              // 最大应力度 / Maximum stress degree
	StressMinScale               float32             `json:"StressMinScale"`               // 最小应力缩放 / Minimum stress scale
	ScaleEaseSpeed               float32             `json:"ScaleEaseSpeed"`               // 缩放平滑速度 / Scale easing speed
	PanierForceDistanceThreshold float32             `json:"PanierForceDistanceThreshold"` // 裙撑力度距离阈值 / Panier force distance threshold
	CalcTime                     int32               `json:"CalcTime"`                     // 计算时间 / Calculation time
	VelocityForceRate            float32             `json:"VelocityForceRate"`            // 速度力率 / Velocity force rate
	VelocityForceRateDistrib     AnimationCurve      `json:"VelocityForceRateDistrib"`     // 速度力率分布曲线 / Velocity force rate distribution curve
	Gravity                      Vector3             `json:"Gravity"`                      // 重力向量 / Gravity vector
	GravityDistrib               AnimationCurve      `json:"GravityDistrib"`               // 重力分布曲线 / Gravity distribution curve
	HardValues                   [4]float32          `json:"HardValues"`                   // 硬度值数组 / Hardness value array
}

// PanierRadiusGroup 存储骨骼特定的半径信息
// PanierRadiusGroup stores bone-specific radius information
type PanierRadiusGroup struct {
	BoneName string         `json:"BoneName"` // 骨骼名称 / Bone name
	Radius   float32        `json:"Radius"`   // 该骨骼的裙撑半径 / Panier radius for this bone
	Curve    AnimationCurve `json:"Curve"`    // 半径分布曲线 / Radius distribution curve
}

// ReadPsk 读取并解析一个 .psk 文件，返回 Psk 结构
// ReadPsk reads and parses a .psk file and returns a Psk structure
func ReadPsk(r io.Reader) (*Psk, error) {
	psk := &Psk{}

	reader := stream.NewBinaryReader(r)

	// 1. 读取签名字符串 "CM3D21_PSK"
	// 1. Read the "CM3D21_PSK" signature string
	sig, err := reader.ReadString()
	if err != nil {
		return nil, fmt.Errorf("read psk signature failed: %w", err)
	}
	// if sig != "CM3D21_PSK" {
	// 	return nil, fmt.Errorf("invalid .psk signature: got %q, want CM3D21_PSK", sig)
	// }
	psk.Signature = sig

	// 2. 读取版本号 int32
	// 2. Read the int32 version
	ver, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read psk version failed: %w", err)
	}
	psk.Version = ver

	// 3. 读取裙撑半径
	// 3. Read the panier radius
	panierRadius, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read panier radius failed: %w", err)
	}
	psk.PanierRadius = panierRadius

	// 4. 读取裙撑半径分布曲线
	// 4. Read the panier radius distribution curve
	panierRadiusDistrib, err := ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read panier radius distribution curve failed: %w", err)
	}
	psk.PanierRadiusDistrib = panierRadiusDistrib

	// 5. 判断版本号并读取裙撑半径分布组
	// 游戏源码只以 217 为门槛，没有说明该值与发行版本号的对应关系
	// 5. Check the version and read the panier radius distribution groups
	// The game source uses 217 as the threshold without documenting its mapping to a release version
	if ver >= 217 {
		groupCount, err := reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read panier radius group count failed: %w", err)
		}
		if err := validateNonNegativeCount("panier radius group count", groupCount); err != nil {
			return nil, err
		}

		if groupCount > 0 {
			psk.PanierRadiusDistribGroups = makeCountedSliceForAppend[PanierRadiusGroup](groupCount)
			for i := int32(0); i < groupCount; i++ {
				boneName, err := reader.ReadString()
				if err != nil {
					return nil, fmt.Errorf("read bone name failed: %w", err)
				}

				radius, err := reader.ReadFloat32()
				if err != nil {
					return nil, fmt.Errorf("read radius failed: %w", err)
				}

				curve, err := ReadAnimationCurve(reader)
				if err != nil {
					return nil, fmt.Errorf("read curve failed: %w", err)
				}

				psk.PanierRadiusDistribGroups = append(psk.PanierRadiusDistribGroups, PanierRadiusGroup{
					BoneName: boneName,
					Radius:   radius,
					Curve:    curve,
				})
			}
		}
	}

	// 6. 读取裙撑力度
	// 6. Read the panier force
	panierForce, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read panier force failed: %w", err)
	}
	psk.PanierForce = panierForce

	// 7. 读取裙撑力度分布曲线
	// 7. Read the panier force distribution curve
	panierForceDistrib, err := ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read panier force distribution curve failed: %w", err)
	}
	psk.PanierForceDistrib = panierForceDistrib

	// 8. 读取裙撑应力
	// 8. Read the panier stress force
	panierStressForce, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read panier stress force failed: %w", err)
	}
	psk.PanierStressForce = panierStressForce

	// 9. 读取最小应力度
	// 9. Read the minimum stress degree
	stressDegreeMin, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read stress degree min failed: %w", err)
	}
	psk.StressDegreeMin = stressDegreeMin

	// 10. 读取最大应力度
	// 10. Read the maximum stress degree
	stressDegreeMax, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read stress degree max failed: %w", err)
	}
	psk.StressDegreeMax = stressDegreeMax

	// 11. 读取最小应力缩放
	// 11. Read the minimum stress scale
	stressMinScale, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read stress min scale failed: %w", err)
	}
	psk.StressMinScale = stressMinScale

	// 12. 读取缩放平滑速度
	// 12. Read the scale easing speed
	scaleEaseSpeed, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read scale ease speed failed: %w", err)
	}
	psk.ScaleEaseSpeed = scaleEaseSpeed

	// 13. 读取裙撑力度距离阈值
	// 13. Read the panier force distance threshold
	panierForceDistanceThreshold, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read panier force distance threshold failed: %w", err)
	}
	psk.PanierForceDistanceThreshold = panierForceDistanceThreshold

	// 14. 读取计算时间
	// 14. Read the calculation time
	calcTime, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read calc time failed: %w", err)
	}
	psk.CalcTime = calcTime

	// 15. 读取速度力率
	// 15. Read the velocity force rate
	velocityForceRate, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read velocity force rate failed: %w", err)
	}
	psk.VelocityForceRate = velocityForceRate

	// 16. 读取速度力率分布曲线
	// 16. Read the velocity force rate distribution curve
	velocityForceRateDistrib, err := ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read velocity force rate distribution curve failed: %w", err)
	}
	psk.VelocityForceRateDistrib = velocityForceRateDistrib

	// 17. 读取重力向量
	// 17. Read the gravity vector
	gravityX, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read gravity X failed: %w", err)
	}
	gravityY, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read gravity Y failed: %w", err)
	}
	gravityZ, err := reader.ReadFloat32()
	if err != nil {
		return nil, fmt.Errorf("read gravity Z failed: %w", err)
	}
	psk.Gravity = Vector3{X: gravityX, Y: gravityY, Z: gravityZ}

	// 18. 读取重力分布曲线
	// 18. Read the gravity distribution curve
	gravityDistrib, err := ReadAnimationCurve(reader)
	if err != nil {
		return nil, fmt.Errorf("read gravity distribution curve failed: %w", err)
	}
	psk.GravityDistrib = gravityDistrib

	// 19. 读取硬度值数组
	// 19. Read the hardness value array
	for i := 0; i < 4; i++ {
		hardValue, err := reader.ReadFloat32()
		if err != nil {
			return nil, fmt.Errorf("read hard value %d failed: %w", i, err)
		}
		psk.HardValues[i] = hardValue
	}

	return psk, nil
}

// Dump 将 Psk 结构写到 w 中，生成符合 CM3D21_PSK 格式的二进制数据
// Dump writes a Psk structure to w as CM3D21_PSK binary data
func (p Psk) Dump(w io.Writer) error {
	if err := validatePskForDump(&p); err != nil {
		return err
	}
	writer := stream.NewBinaryWriter(w)

	// 1. 写签名
	// 1. Write the signature
	if err := writer.WriteString(p.Signature); err != nil {
		return fmt.Errorf("write psk signature failed: %w", err)
	}

	// 2. 写版本号
	// 2. Write the version
	if err := writer.WriteInt32(p.Version); err != nil {
		return fmt.Errorf("write psk version failed: %w", err)
	}

	// 3. 写裙撑半径
	// 3. Write the panier radius
	if err := writer.WriteFloat32(p.PanierRadius); err != nil {
		return fmt.Errorf("write panier radius failed: %w", err)
	}

	// 4. 写裙撑半径分布曲线
	// 4. Write the panier radius distribution curve
	if err := WriteAnimationCurve(writer, p.PanierRadiusDistrib); err != nil {
		return fmt.Errorf("write panier radius distribution curve failed: %w", err)
	}

	// 5. 写裙撑半径分布组（版本 217 起才存在）
	// 5. Write the panier radius distribution groups, which exist from version 217
	if p.Version >= 217 {
		groupCount := int32(len(p.PanierRadiusDistribGroups))
		if err := writer.WriteInt32(groupCount); err != nil {
			return fmt.Errorf("write panier radius group count failed: %w", err)
		}

		for i := int32(0); i < groupCount; i++ {
			group := p.PanierRadiusDistribGroups[i]
			if err := writer.WriteString(group.BoneName); err != nil {
				return fmt.Errorf("write bone name failed: %w", err)
			}

			if err := writer.WriteFloat32(group.Radius); err != nil {
				return fmt.Errorf("write radius failed: %w", err)
			}

			if err := WriteAnimationCurve(writer, group.Curve); err != nil {
				return fmt.Errorf("write curve failed: %w", err)
			}
		}
	}

	// 6. 写裙撑力度
	// 6. Write the panier force
	if err := writer.WriteFloat32(p.PanierForce); err != nil {
		return fmt.Errorf("write panier force failed: %w", err)
	}

	// 7. 写裙撑力度分布曲线
	// 7. Write the panier force distribution curve
	if err := WriteAnimationCurve(writer, p.PanierForceDistrib); err != nil {
		return fmt.Errorf("write panier force distribution curve failed: %w", err)
	}

	// 8. 写裙撑应力
	// 8. Write the panier stress force
	if err := writer.WriteFloat32(p.PanierStressForce); err != nil {
		return fmt.Errorf("write panier stress force failed: %w", err)
	}

	// 9. 写最小应力度
	// 9. Write the minimum stress degree
	if err := writer.WriteFloat32(p.StressDegreeMin); err != nil {
		return fmt.Errorf("write stress degree min failed: %w", err)
	}

	// 10. 写最大应力度
	// 10. Write the maximum stress degree
	if err := writer.WriteFloat32(p.StressDegreeMax); err != nil {
		return fmt.Errorf("write stress degree max failed: %w", err)
	}

	// 11. 写最小应力缩放
	// 11. Write the minimum stress scale
	if err := writer.WriteFloat32(p.StressMinScale); err != nil {
		return fmt.Errorf("write stress min scale failed: %w", err)
	}

	// 12. 写缩放平滑速度
	// 12. Write the scale easing speed
	if err := writer.WriteFloat32(p.ScaleEaseSpeed); err != nil {
		return fmt.Errorf("write scale ease speed failed: %w", err)
	}

	// 13. 写裙撑力度距离阈值
	// 13. Write the panier force distance threshold
	if err := writer.WriteFloat32(p.PanierForceDistanceThreshold); err != nil {
		return fmt.Errorf("write panier force distance threshold failed: %w", err)
	}

	// 14. 写计算时间
	// 14. Write the calculation time
	if err := writer.WriteInt32(p.CalcTime); err != nil {
		return fmt.Errorf("write calc time failed: %w", err)
	}

	// 15. 写速度力率
	// 15. Write the velocity force rate
	if err := writer.WriteFloat32(p.VelocityForceRate); err != nil {
		return fmt.Errorf("write velocity force rate failed: %w", err)
	}

	// 16. 写速度力率分布曲线
	// 16. Write the velocity force rate distribution curve
	if err := WriteAnimationCurve(writer, p.VelocityForceRateDistrib); err != nil {
		return fmt.Errorf("write velocity force rate distribution curve failed: %w", err)
	}

	// 17. 写重力向量
	// 17. Write the gravity vector
	if err := writer.WriteFloat32(p.Gravity.X); err != nil {
		return fmt.Errorf("write gravity X failed: %w", err)
	}
	if err := writer.WriteFloat32(p.Gravity.Y); err != nil {
		return fmt.Errorf("write gravity Y failed: %w", err)
	}
	if err := writer.WriteFloat32(p.Gravity.Z); err != nil {
		return fmt.Errorf("write gravity Z failed: %w", err)
	}

	// 18. 写重力分布曲线
	// 18. Write the gravity distribution curve
	if err := WriteAnimationCurve(writer, p.GravityDistrib); err != nil {
		return fmt.Errorf("write gravity distribution curve failed: %w", err)
	}

	// 19. 写硬度值数组
	// 19. Write the hardness value array
	for i := 0; i < 4; i++ {
		if err := writer.WriteFloat32(p.HardValues[i]); err != nil {
			return fmt.Errorf("write hard value %d failed: %w", i, err)
		}
	}

	return nil
}

// validatePskForDump 验证版本门槛、组数量和所有曲线关键帧数量可在线格式中表示
// validatePskForDump verifies that the version gate, group count, and all curve keyframe counts are representable in the wire format
func validatePskForDump(psk *Psk) error {
	if psk.Version < 217 && len(psk.PanierRadiusDistribGroups) != 0 {
		return fmt.Errorf("psk version %d cannot encode %d panier radius groups", psk.Version, len(psk.PanierRadiusDistribGroups))
	}
	if psk.Version >= 217 {
		if _, err := collectionCountInt32("panier radius group count", int64(len(psk.PanierRadiusDistribGroups))); err != nil {
			return err
		}
	}
	curves := []struct {
		name  string         // 曲线名称 / Curve name
		curve AnimationCurve // 曲线数据 / Curve data
	}{
		{name: "PanierRadiusDistrib", curve: psk.PanierRadiusDistrib},
		{name: "PanierForceDistrib", curve: psk.PanierForceDistrib},
		{name: "VelocityForceRateDistrib", curve: psk.VelocityForceRateDistrib},
		{name: "GravityDistrib", curve: psk.GravityDistrib},
	}
	if psk.Version >= 217 {
		for index, group := range psk.PanierRadiusDistribGroups {
			curves = append(curves, struct {
				name  string         // 曲线名称 / Curve name
				curve AnimationCurve // 曲线数据 / Curve data
			}{name: fmt.Sprintf("PanierRadiusDistribGroups[%d].Curve", index), curve: group.Curve})
		}
	}
	for _, curve := range curves {
		if _, err := collectionCountInt32(curve.name+" keyCount", int64(len(curve.curve.Keyframes))); err != nil {
			return err
		}
	}
	return nil
}

package application

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	archivePageTokenVersion = byte(1)
	archivePageTokenMACSize = sha256.Size
)

// ArchivePager 使用进程本地随机密钥签名不透明归档分页游标 / ArchivePager signs opaque archive page cursors with a process-local random key
type ArchivePager struct {
	// key 是当前分页器用于签名和验证游标的密钥 / key is the key used by this pager to sign and verify cursors
	key [sha256.Size]byte
}

// NewArchivePager 创建具有随机签名密钥的归档分页器
// NewArchivePager creates an archive pager with a random signing key
func NewArchivePager() (*ArchivePager, error) {
	pager := &ArchivePager{}
	if _, err := rand.Read(pager.key[:]); err != nil {
		return nil, fmt.Errorf("generate archive page-token key: %w", err)
	}
	return pager, nil
}

// ArchiveListing 表示经过排序和资源限制检查的单个归档视图 / ArchiveListing represents one sorted archive view that has passed resource-limit checks
type ArchiveListing struct {
	// FormatID 是生成列表的归档格式标识符 / FormatID is the archive format identifier used to produce the listing
	FormatID string
	// Entries 是按名称排序的归档条目 / Entries contains archive entries sorted by name
	Entries []ArchiveEntry
	// fingerprint 将分页游标绑定到格式、源文件内容和条目元数据 / fingerprint binds page cursors to the format, source bytes, and entry metadata
	fingerprint [sha256.Size]byte
}

// Decode 验证分页游标并返回其在当前归档列表中的偏移量
// Decode validates a page cursor and returns its offset within the current archive listing
func (p *ArchivePager) Decode(listing ArchiveListing, token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	if len(token) > 512 {
		return 0, archivePageTokenError("token is too long")
	}
	if p == nil {
		return 0, archivePageTokenError("pager is unavailable")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != token {
		return 0, archivePageTokenError("token encoding is invalid")
	}
	const fixedPayload = 1 + 8 + 2 + sha256.Size
	if len(raw) < fixedPayload+archivePageTokenMACSize {
		return 0, archivePageTokenError("token is truncated")
	}
	formatLength := int(binary.BigEndian.Uint16(raw[9:11]))
	payloadLength := fixedPayload + formatLength
	if len(raw) != payloadLength+archivePageTokenMACSize {
		return 0, archivePageTokenError("token length is invalid")
	}
	payload, suppliedMAC := raw[:payloadLength], raw[payloadLength:]
	mac := hmac.New(sha256.New, p.key[:])
	_, _ = mac.Write(payload)
	if !hmac.Equal(suppliedMAC, mac.Sum(nil)) {
		return 0, archivePageTokenError("token signature is invalid")
	}
	if payload[0] != archivePageTokenVersion {
		return 0, archivePageTokenError("token version is unsupported")
	}
	formatID := string(payload[11 : 11+formatLength])
	if formatID != normalizeArchiveFormatID(listing.FormatID) {
		return 0, archivePageTokenError("token belongs to another format")
	}
	fingerprint := payload[11+formatLength : payloadLength]
	if !hmac.Equal(fingerprint, listing.fingerprint[:]) {
		return 0, archivePageTokenError("archive listing changed")
	}
	offset64 := binary.BigEndian.Uint64(payload[1:9])
	if offset64 > uint64(math.MaxInt) || int(offset64) >= len(listing.Entries) {
		return 0, archivePageTokenError("token offset is outside the current listing")
	}
	return int(offset64), nil
}

// Encode 为当前归档列表中的后续偏移量生成签名分页游标
// Encode creates a signed page cursor for a subsequent offset in the current archive listing
func (p *ArchivePager) Encode(listing ArchiveListing, offset int) (string, error) {
	if p == nil {
		return "", archivePageTokenError("pager is unavailable")
	}
	if offset <= 0 || offset >= len(listing.Entries) {
		return "", archivePageTokenError("next offset is outside the current listing")
	}
	formatID := normalizeArchiveFormatID(listing.FormatID)
	if formatID == "" || len(formatID) > math.MaxUint16 {
		return "", archivePageTokenError("format ID is invalid")
	}
	payload := make([]byte, 1+8+2+len(formatID)+sha256.Size)
	payload[0] = archivePageTokenVersion
	binary.BigEndian.PutUint64(payload[1:9], uint64(offset))
	binary.BigEndian.PutUint16(payload[9:11], uint16(len(formatID)))
	copy(payload[11:], formatID)
	copy(payload[11+len(formatID):], listing.fingerprint[:])
	mac := hmac.New(sha256.New, p.key[:])
	_, _ = mac.Write(payload)
	raw := append(payload, mac.Sum(nil)...)
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// archiveListingFingerprint 计算绑定归档格式、源摘要和有序条目元数据的摘要
// archiveListingFingerprint computes a digest binding the archive format, source digest, and ordered entry metadata
func archiveListingFingerprint(formatID string, sourceDigest [sha256.Size]byte, entries []ArchiveEntry) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("meido-archive-listing-v1\x00"))
	writeArchiveFingerprintString(hash, normalizeArchiveFormatID(formatID))
	_, _ = hash.Write(sourceDigest[:])
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(entries)))
	_, _ = hash.Write(count[:])
	for _, entry := range entries {
		writeArchiveFingerprintString(hash, entry.Name)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(entry.Size))
		_, _ = hash.Write(size[:])
		writeArchiveFingerprintString(hash, entry.Kind)
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

// archiveFingerprintWriter 定义归档列表指纹编码所需的写入操作 / archiveFingerprintWriter defines the write operation required to encode an archive-listing fingerprint
type archiveFingerprintWriter interface {
	// Write 将字节加入正在计算的指纹
	// Write adds bytes to the fingerprint being computed
	Write([]byte) (int, error)
}

// writeArchiveFingerprintString 以长度前缀编码字符串并写入归档指纹
// writeArchiveFingerprintString writes a length-prefixed string into an archive fingerprint
func writeArchiveFingerprintString(writer archiveFingerprintWriter, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write([]byte(value))
}

// normalizeArchiveFormatID 规范化用于归档路由和分页游标绑定的格式标识符
// normalizeArchiveFormatID normalizes a format identifier for archive routing and cursor binding
func normalizeArchiveFormatID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// archivePageTokenError 创建公开代码为参数无效的归档分页游标错误
// archivePageTokenError creates an archive page-cursor error with the public invalid-argument code
func archivePageTokenError(reason string) error {
	return opError("archive page token", CodeInvalidArgument, fmt.Errorf("invalid archive page_token: %s", reason))
}

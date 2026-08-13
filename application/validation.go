package application

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	editingv1 "github.com/MeidoPromotionAssociation/MeidoSerialization/schemas/editing/v1"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// validateEditingJSONPath 在转换器读取编辑 JSON 前按照已发布模式检查结构和精确数值边界
// validateEditingJSONPath checks structure and exact numeric bounds against the published schema before a converter reads editing JSON
func (e *Engine) validateEditingJSONPath(ctx context.Context, path, formatID string) error {
	if err := ctx.Err(); err != nil {
		return opError("validate editing JSON", CodeCanceled, err)
	}
	file, err := os.Open(path)
	if err != nil {
		return opError("validate editing JSON", CodeInternal, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	hasBOM := false
	if prefix, peekErr := reader.Peek(3); peekErr == nil && bytes.Equal(prefix, []byte{0xef, 0xbb, 0xbf}) {
		hasBOM = true
		if _, discardErr := reader.Discard(3); discardErr != nil {
			return opError("validate editing JSON", CodeInternal, discardErr)
		}
	}
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	var instance any
	if err := decoder.Decode(&instance); err != nil {
		return opError("validate editing JSON", CodeInvalidArgument, fmt.Errorf("decode %s: %w; this input must already be the editing JSON representation of %s rather than native game data, so convert native data to editing JSON before requesting a native target", formatID, err, formatID))
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return opError("validate editing JSON", CodeInvalidArgument, fmt.Errorf("%s contains more than one JSON value", formatID))
		}
		return opError("validate editing JSON", CodeInvalidArgument, fmt.Errorf("trailing content in %s: %w", formatID, err))
	}

	document, found, err := editingv1.Lookup(formatID)
	if err != nil {
		return opError("validate editing JSON", CodeInternal, err)
	}
	if !found {
		return opError("validate editing JSON", CodeUnsupported, fmt.Errorf("format %q has no published editing schema", formatID))
	}
	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(document.JSON))
	if err != nil {
		return opError("validate editing JSON", CodeInternal, fmt.Errorf("decode published schema: %w", err))
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertContent()
	if err := compiler.AddResource(document.ID, schemaDocument); err != nil {
		return opError("validate editing JSON", CodeInternal, fmt.Errorf("load published schema: %w", err))
	}
	resolved, err := compiler.Compile(document.ID)
	if err != nil {
		return opError("validate editing JSON", CodeInternal, fmt.Errorf("compile published schema: %w", err))
	}
	if err := resolved.Validate(instance); err != nil {
		return opError("validate editing JSON", CodeInvalidArgument, fmt.Errorf("%s does not match published schema: %w", formatID, err))
	}
	if hasBOM {
		if err := stripEditingJSONBOM(ctx, path); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return opError("validate editing JSON", CodeCanceled, ctxErr)
			}
			return opError("validate editing JSON", CodeInternal, fmt.Errorf("normalize UTF-8 BOM: %w", err))
		}
	}
	return nil
}

// stripEditingJSONBOM 就地移除编辑 JSON 开头的 UTF-8 BOM 并保留其余内容
// stripEditingJSONBOM removes a leading UTF-8 BOM from editing JSON in place while preserving the remaining content
func stripEditingJSONBOM(ctx context.Context, path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	var prefix [3]byte
	if _, err := io.ReadFull(file, prefix[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		return err
	}
	if !bytes.Equal(prefix[:], []byte{0xef, 0xbb, 0xbf}) {
		return nil
	}

	buffer := make([]byte, 64<<10)
	readOffset, writeOffset := int64(len(prefix)), int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := file.ReadAt(buffer, readOffset)
		if n > 0 {
			written, writeErr := file.WriteAt(buffer[:n], writeOffset)
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
			readOffset += int64(n)
			writeOffset += int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	if err := file.Truncate(writeOffset); err != nil {
		return err
	}
	return file.Sync()
}

package COM3D2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/conversionio"
)

func checkConversionContext(ctx context.Context) error {
	return conversionio.Check(ctx)
}

func readConversionJSON(ctx context.Context, path string, value any) error {
	data, err := conversionio.ReadFile(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return err
	}
	return conversionio.Check(ctx)
}

func writeConversionJSON(ctx context.Context, path string, value any, maxOutputBytes int64) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := conversionio.Check(ctx); err != nil {
		return err
	}
	return conversionio.WriteFile(ctx, path, data, 0644, maxOutputBytes)
}

func writeConversionBinary(
	ctx context.Context,
	path string,
	maxOutputBytes int64,
	dump func(io.Writer) error,
) error {
	f, err := conversionio.CreateFile(ctx, path, 0644, maxOutputBytes)
	if err != nil {
		return err
	}
	dumpErr := dump(f)
	closeErr := f.Close()
	if dumpErr != nil {
		return dumpErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := conversionio.Check(ctx); err != nil {
		return err
	}
	return nil
}

func conversionOutputError(kind string, err error) error {
	return fmt.Errorf("write %s conversion output: %w", kind, err)
}

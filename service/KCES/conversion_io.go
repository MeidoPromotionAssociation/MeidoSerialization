package KCES

import (
	"context"
	"io"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/internal/conversionio"
)

func checkConversionContext(ctx context.Context) error {
	return conversionio.Check(ctx)
}

func readConversionFile(ctx context.Context, path string) ([]byte, error) {
	return conversionio.ReadFile(ctx, path)
}

func writeConversionFile(ctx context.Context, path string, data []byte, maxOutputBytes int64) error {
	return conversionio.WriteFile(ctx, path, data, 0644, maxOutputBytes)
}

func createConversionFile(ctx context.Context, path string, maxOutputBytes int64) (io.WriteCloser, error) {
	return conversionio.CreateFile(ctx, path, 0644, maxOutputBytes)
}

func newConversionBudget(ctx context.Context, maxOutputBytes int64) (*conversionio.Budget, error) {
	return conversionio.NewBudget(ctx, maxOutputBytes)
}

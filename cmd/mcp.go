package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/application"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/transport/mcpserver"
	"github.com/spf13/cobra"
)

// newMCPCmd 构建并配置通过标准输入输出运行的 MCP 服务器命令
// newMCPCmd builds and configures the MCP server command that runs over standard input and output
func newMCPCmd() *cobra.Command {
	var (
		rootSpecs      []string
		writeRootSpecs []string
		restrictPaths  bool
		maxResultMiB   int64
		maxWriteMiB    int64
	)
	command := &cobra.Command{
		Use:   "mcp",
		Short: "Run the Model Context Protocol server over stdio",
		Long: "Run the Model Context Protocol server over stdio. With no root flags, " +
			"file tools accept unrestricted path/output_path values. Configure a root or " +
			"use --restrict-paths to enable confined root-ID mode.",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			filesystemMode := mcpFilesystemMode(restrictPaths, rootSpecs, writeRootSpecs)
			roots, err := configuredRootsWithWrites(rootSpecs, writeRootSpecs)
			if err != nil {
				return err
			}
			defer roots.Close()
			maxResultBytes, err := mebibytes(maxResultMiB)
			if err != nil {
				return err
			}
			maxWriteBytes, err := mebibytes(maxWriteMiB)
			if err != nil {
				return err
			}
			engine := application.NewEngine(application.EngineOptions{MaxInputBytes: maxWriteBytes, MaxOutputBytes: maxWriteBytes})
			logger := slog.New(slog.NewTextHandler(command.ErrOrStderr(), nil))
			if filesystemMode == mcpserver.FilesystemModeUnrestricted {
				logger.Warn("MCP filesystem restrictions are disabled; file tools can access any path allowed by the process account")
			}
			server, err := mcpserver.New(mcpserver.Config{
				Engine: engine, Roots: roots, FilesystemMode: filesystemMode, Logger: logger, Version: applicationVersion(),
				MaxResultBytes: maxResultBytes, MaxWriteBytes: maxWriteBytes,
			})
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			err = server.Run(ctx)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		},
	}
	command.Flags().StringArrayVar(&rootSpecs, "root", nil, "restrict read access to id=directory (repeatable; enables restricted mode)")
	command.Flags().StringArrayVar(&writeRootSpecs, "write-root", nil, "restrict output to id=directory (repeatable; also readable; enables restricted mode)")
	command.Flags().BoolVar(&restrictPaths, "restrict-paths", false, "restrict file access to --root/--write-root entries (root flags enable this automatically)")
	command.Flags().Int64Var(&maxResultMiB, "max-result-mib", 2, "maximum editing JSON returned inline to the model")
	command.Flags().Int64Var(&maxWriteMiB, "max-write-mib", 512, "maximum converted or extracted file size")
	return command
}

// mcpFilesystemMode 根据显式限制和根目录参数选择 MCP 文件系统访问模式
// mcpFilesystemMode selects the MCP filesystem access mode from explicit restrictions and root flags
func mcpFilesystemMode(restrictPaths bool, rootSpecs, writeRootSpecs []string) mcpserver.FilesystemMode {
	if restrictPaths || len(rootSpecs) != 0 || len(writeRootSpecs) != 0 {
		return mcpserver.FilesystemModeRestricted
	}
	return mcpserver.FilesystemModeUnrestricted
}

var mcpCmd = newMCPCmd()

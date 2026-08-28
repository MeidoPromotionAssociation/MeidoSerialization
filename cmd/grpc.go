package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/application"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/internal/blobstore"
	"github.com/MeidoPromotionAssociation/MeidoSerialization/v2/transport/grpcserver"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run MeidoSerialization transport servers",
}

// newGRPCCmd 构建并配置用于运行版本化 gRPC API 的子命令
// newGRPCCmd builds and configures the subcommand that runs the versioned gRPC API
func newGRPCCmd() *cobra.Command {
	var (
		listenAddress string
		rootSpecs     []string
		blobDirectory string
		maxBlobMiB    int64
		maxTotalMiB   int64
		maxBlobs      int
		blobTTL       time.Duration
		inlineMiB     int64
		allowRemote   bool
		restrictPaths bool
	)
	command := &cobra.Command{
		Use:   "grpc",
		Short: "Run the versioned protobuf/gRPC API",
		Long: "Run the versioned protobuf/gRPC API. With no --root or --restrict-paths flag, ArtifactInput.path " +
			"accepts direct server-local paths. Configure a root or use --restrict-paths to " +
			"enable confined root-ID mode.",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := validateGRPCListenAddress(listenAddress, allowRemote); err != nil {
				return err
			}
			roots, err := configuredRoots(rootSpecs)
			if err != nil {
				return err
			}
			defer roots.Close()
			filesystemMode := grpcFilesystemMode(restrictPaths, rootSpecs)
			maxBlobBytes, err := mebibytes(maxBlobMiB)
			if err != nil {
				return err
			}
			maxTotalBytes, err := mebibytes(maxTotalMiB)
			if err != nil {
				return err
			}
			maxInlineBytes, err := mebibytes(inlineMiB)
			if err != nil {
				return err
			}
			if maxInlineBytes > grpcserver.MaxInlineBytes {
				return fmt.Errorf("--inline-mib must not exceed 3")
			}
			blobs, err := blobstore.New(blobstore.Config{
				Directory: blobDirectory, MaxBlobBytes: maxBlobBytes,
				MaxTotalBytes: maxTotalBytes, MaxBlobs: maxBlobs, TTL: blobTTL,
			})
			if err != nil {
				return err
			}
			defer blobs.Close()
			engine := application.NewEngine(application.EngineOptions{MaxInputBytes: maxBlobBytes, MaxOutputBytes: maxBlobBytes})
			api, err := grpcserver.New(grpcserver.Config{
				Engine: engine, Roots: roots, FilesystemMode: filesystemMode, Blobs: blobs, MaxInlineBytes: maxInlineBytes,
			})
			if err != nil {
				return err
			}
			listener, err := net.Listen("tcp", listenAddress)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", listenAddress, err)
			}
			defer listener.Close()

			server := grpc.NewServer(
				grpc.MaxRecvMsgSize(4<<20),
				grpc.MaxSendMsgSize(4<<20),
			)
			api.Register(server)
			healthServer := health.NewServer()
			grpc_health_v1.RegisterHealthServer(server, healthServer)
			healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
			reflection.Register(server)

			logger := slog.New(slog.NewTextHandler(command.ErrOrStderr(), nil))
			if filesystemMode == grpcserver.FilesystemModeUnrestricted {
				logger.Warn("gRPC filesystem restrictions are disabled; path inputs can read any regular file allowed by the process account")
			}
			logger.Info("gRPC server listening", "address", listener.Addr().String(), "filesystem_mode", filesystemMode, "roots", roots.IDs())
			ctx, stopSignals := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stopSignals()
			serveContext, cancelServe := context.WithCancel(ctx)
			defer cancelServe()
			go func() {
				<-serveContext.Done()
				stopped := make(chan struct{})
				go func() {
					server.GracefulStop()
					close(stopped)
				}()
				select {
				case <-stopped:
				case <-time.After(5 * time.Second):
					server.Stop()
				}
			}()
			err = server.Serve(listener)
			cancelServe()
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				return err
			}
			return nil
		},
	}
	command.Flags().StringVar(&listenAddress, "listen", "127.0.0.1:50051", "TCP address to listen on")
	command.Flags().StringArrayVar(&rootSpecs, "root", nil, "restrict file access to id=directory (repeatable; enables restricted mode)")
	command.Flags().BoolVar(&restrictPaths, "restrict-paths", false, "restrict file access to --root entries (root flags enable this automatically)")
	command.Flags().StringVar(&blobDirectory, "blob-dir", "", "exclusive blob directory (empty uses a process-owned temporary directory)")
	command.Flags().Int64Var(&maxBlobMiB, "max-blob-mib", 4096, "maximum size of one streamed blob")
	command.Flags().Int64Var(&maxTotalMiB, "max-total-blob-mib", 16384, "maximum total temporary blob storage")
	command.Flags().IntVar(&maxBlobs, "max-blobs", blobstore.DefaultMaxBlobs, "maximum number of stored and in-flight blobs")
	command.Flags().DurationVar(&blobTTL, "blob-ttl", blobstore.DefaultTTL, "temporary blob lifetime")
	command.Flags().Int64Var(&inlineMiB, "inline-mib", 3, "maximum unary inline payload size (at most 3 MiB)")
	command.Flags().BoolVar(&allowRemote, "allow-remote", false, "allow an unencrypted listener on a non-loopback address")
	return command
}

// grpcFilesystemMode 根据显式限制和根目录参数选择 gRPC 文件系统访问模式
// grpcFilesystemMode selects the gRPC filesystem access mode from explicit restrictions and root flags
func grpcFilesystemMode(restrictPaths bool, rootSpecs []string) grpcserver.FilesystemMode {
	if restrictPaths || len(rootSpecs) != 0 {
		return grpcserver.FilesystemModeRestricted
	}
	return grpcserver.FilesystemModeUnrestricted
}

// validateGRPCListenAddress 校验监听地址并默认拒绝未授权的非回环端点
// validateGRPCListenAddress validates a listen address and rejects unauthorized non-loopback endpoints by default
func validateGRPCListenAddress(address string, allowRemote bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid --listen address %q: %w", address, err)
	}
	if allowRemote || strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("refusing non-loopback --listen address %q without --allow-remote; this server does not configure TLS or authentication", address)
	}
	return nil
}

// init 将 gRPC 服务器命令注册到传输服务命令组
// init registers the gRPC server command with the transport-service command group
func init() {
	serveCmd.AddCommand(newGRPCCmd())
}

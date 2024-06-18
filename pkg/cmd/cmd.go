package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"harbor-migrate/pkg/log"
	"harbor-migrate/version"
	urlpkg "net/url"
	"os"
	"strings"
)

var logger log.Logger

func init() {
	logger = log.NewLogger("harbor-migrate")
}

// HarborMigrateOptions contains configuration flags for the HarborMigrate.
type HarborMigrateOptions struct {
	Source HarborConfig
	Target HarborConfig
}

// validateHarborMigrateOptions validates HarborMigrate configuration flags and returns an error if they are invalid.
func validateHarborMigrateOptions(opt *HarborMigrateOptions) (err error) {
	sourceUrl, err := urlpkg.Parse(opt.Source.URL)
	if err != nil {
		return err
	}
	targetUrl, err := urlpkg.Parse(opt.Target.URL)
	if err != nil {
		return err
	}
	if strings.EqualFold(sourceUrl.String(), targetUrl.String()) {
		return fmt.Errorf("The same 'source' and 'target', No-op\n")
	}
	return nil
}

// NewHarborMigrateCommand creates a *cobra.Command object with default parameters
func NewHarborMigrateCommand() *cobra.Command {
	options := &HarborMigrateOptions{}

	flags := pflag.CommandLine
	flags.BoolP("help", "h", false, "帮助信息")
	flags.StringVar(&options.Source.URL, "source-url", "https://pcr.io", "源地址")
	flags.StringVar(&options.Source.Username, "source-user", "admin", "源用户名")
	flags.StringVar(&options.Source.Password, "source-pass", "Harbor12345", "源密码")
	flags.StringVar(&options.Target.URL, "target-url", "https://3.pcr.io", "目的地址")
	flags.StringVar(&options.Target.Username, "target-user", "admin", "目的用户名")
	flags.StringVar(&options.Target.Password, "target-pass", "Harbor12345", "目的密码")

	cmd := &cobra.Command{
		Use:                os.Args[0],
		Long:               `很长的描述`,
		DisableFlagParsing: false,
		Run: func(cmd *cobra.Command, args []string) {
			if err := flags.Parse(args); err != nil {
				logger.Error("Failed to parse flag", err)
				_ = cmd.Usage()
				os.Exit(1)
			}

			// check if there are non-flag arguments in the command line
			cmds := flags.Args()
			if len(cmds) > 0 {
				logger.Errorf("Unknown command %s", cmds[0])
				_ = cmd.Usage()
				os.Exit(1)
			}

			// short-circuit on help
			help, err := flags.GetBool("help")
			if err != nil {
				logger.Info(`"help" flag is non-bool, programmer error, please correct`)
				os.Exit(1)
			}
			if help {
				_ = cmd.Help()
				return
			}

			if err := validateHarborMigrateOptions(options); err != nil {
				logger.Errorf("Validate options failed: %v", err)
				os.Exit(1)
			}

			if err := run(SetupSignalContext(), options); err != nil {
				if errors.Is(err, context.Canceled) {
					logger.Infof("HarborMigrate canceled")
				} else {
					logger.Errorf("HarborMigrate failed: %v", err)
				}
				os.Exit(1)
			}
		},
	}

	return cmd
}

func run(ctx context.Context, opt *HarborMigrateOptions) error {
	logger.Infof("HarborMigrate version %s", version.Full())

	newCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	// 迁移项目信息
	logger.Info("transfer projects start")
	if err := transferProjects(newCtx, opt.Source, opt.Target); err != nil {
		panic(err)
	}
	logger.Info("transfer projects end")

	// 迁移镜像
	logger.Info("transfer images start")
	transferImages(newCtx, opt.Source, opt.Target)
	logger.Info("transfer images end")

	return nil
}

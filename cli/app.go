package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gruntwork-io/go-commons/version"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/pkg/errors"
	"github.com/gruntwork-io/terragrunt/util"
	hashicorpversion "github.com/hashicorp/go-version"

	awsproviderpatch "github.com/gruntwork-io/terragrunt/cli/commands/aws-provider-patch"
	graphdependencies "github.com/gruntwork-io/terragrunt/cli/commands/graph-dependencies"
	"github.com/gruntwork-io/terragrunt/cli/commands/hclfmt"
	renderjson "github.com/gruntwork-io/terragrunt/cli/commands/render-json"
	runall "github.com/gruntwork-io/terragrunt/cli/commands/run-all"
	"github.com/gruntwork-io/terragrunt/cli/commands/terraform"
	terragruntinfo "github.com/gruntwork-io/terragrunt/cli/commands/terragrunt-info"
	validateinputs "github.com/gruntwork-io/terragrunt/cli/commands/validate-inputs"
	"github.com/gruntwork-io/terragrunt/cli/flags"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/pkg/cli"
	"github.com/gruntwork-io/terragrunt/pkg/env"
	"github.com/gruntwork-io/terragrunt/shell"
)

func init() {
	cli.AppHelpTemplate = appHelpTemplate
	cli.CommandHelpTemplate = commandHelpTemplate
}

// NewApp creates the Terragrunt CLI App.
func NewApp(writer io.Writer, errWriter io.Writer) *cli.App {
	opts := options.NewTerragruntOptions()
	opts.Writer = writer
	opts.ErrWriter = errWriter

	app := cli.NewApp()
	app.Name = "terragrunt"
	app.Usage = "Terragrunt is a thin wrapper for Terraform that provides extra tools for working with multiple\nTerraform modules, remote state, and locking. For documentation, see https://github.com/gruntwork-io/terragrunt/."
	app.Author = "Gruntwork <www.gruntwork.io>"
	app.Version = version.GetVersion()
	app.Writer = writer
	app.ErrWriter = errWriter
	app.Flags = cli.Flags{flags.NewHelpFlag()}
	app.Commands = append(
		newDeprecatedCommands(opts),
		newCommands(opts)...)
	app.Before = beforeRunningCommand(opts)         // all commands refer to this function as `Before`
	app.DefaultCommand = terraform.NewCommand(opts) // by default forwards all commands directly to Terraform
	app.OsExiter = osExiter

	return app
}

// This set of commands is also used in unit tests
func newCommands(opts *options.TerragruntOptions) cli.Commands {
	cmds := cli.Commands{
		runall.NewCommand(opts),            // run-all
		terragruntinfo.NewCommand(opts),    // terragrunt-info
		validateinputs.NewCommand(opts),    // validate-inputs
		graphdependencies.NewCommand(opts), // graph-dependencies
		hclfmt.NewCommand(opts),            // hclfmt
		renderjson.NewCommand(opts),        // render-json
		awsproviderpatch.NewCommand(opts),  // aws-provider-patch
	}

	sort.Sort(cmds)

	// add terraform command `*` after sorting to put the command at the end of the list in the help.
	cmds.Add(terraform.NewCommand(opts))

	return cmds
}

// this function is run for any command
func beforeRunningCommand(opts *options.TerragruntOptions) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		if flagHelp := ctx.Flags.Get(flags.FlagNameHelp); flagHelp.Value().IsSet() {
			err := showHelp(ctx, opts)
			return err
		}

		err := initialSetup(ctx, opts)
		return err
	}
}

func showHelp(ctx *cli.Context, opts *options.TerragruntOptions) error {
	// prevent the command itself from running and exit after displaying help
	ctx.Command.Action = nil

	// If the app command is specified, show help for the command, except for the '*' command as this is the default command.
	if !ctx.Command.IsRoot && ctx.Command.Name != terraform.CommandName {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}

	// If the first argument is a command, it is most likely a terraform command, show Terraform help.
	if cmdName := ctx.Args().CommandName(); cmdName != "" {
		terraformHelpCmd := append([]string{cmdName, "-help"}, ctx.Args().Tail()...)
		return shell.RunTerraformCommand(opts, terraformHelpCmd...)
	}

	// In other cases, show the App help.
	return cli.ShowAppHelp(ctx)
}

// mostly preparing terragrunt options
func initialSetup(ctx *cli.Context, opts *options.TerragruntOptions) error {
	// The env vars are renamed to "..._NO_AUTO_..." in the gobal flags`. These ones are left for backwards compatibility.
	opts.AutoInit = env.GetBoolEnv("TERRAGRUNT_AUTO_INIT", opts.AutoInit)
	opts.AutoRetry = env.GetBoolEnv("TERRAGRUNT_AUTO_RETRY", opts.AutoRetry)
	opts.RunAllAutoApprove = env.GetBoolEnv("TERRAGRUNT_AUTO_APPROVE", opts.RunAllAutoApprove)

	// --- Args
	// convert the rest flags (intended for terraform) to one dash, e.g. `--input=true` to `-input=true`
	args := ctx.Args().Normalize(cli.OneDashFlag).Slice()
	cmdName := ctx.Command.Name

	switch cmdName {
	case terraform.CommandName, runall.CommandName:
		cmdName = ctx.Args().CommandName()
	default:
		args = append([]string{ctx.Command.Name}, args...)
	}

	opts.TerraformCommand = cmdName
	opts.TerraformCliArgs = args

	opts.Env = env.ParseEnvs(os.Environ())

	// --- Logger
	if opts.DisableLogColors {
		util.DisableLogColors()
	}
	opts.LogLevel = util.ParseLogLevel(opts.LogLevelStr)
	opts.Logger = util.CreateLogEntry("", opts.LogLevel)
	opts.Logger.Logger.SetOutput(ctx.App.ErrWriter)

	// --- Working Dir
	if opts.WorkingDir == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return errors.WithStackTrace(err)
		}
		opts.WorkingDir = currentDir
	}
	opts.WorkingDir = filepath.ToSlash(opts.WorkingDir)

	// --- Download Dir
	if opts.DownloadDir == "" {
		opts.DownloadDir = util.JoinPath(opts.WorkingDir, options.TerragruntCacheDir)
	}

	downloadDir, err := filepath.Abs(opts.DownloadDir)
	if err != nil {
		return errors.WithStackTrace(err)
	}
	opts.DownloadDir = filepath.ToSlash(downloadDir)

	// --- Terragrunt ConfigPath
	if opts.TerragruntConfigPath == "" {
		opts.TerragruntConfigPath = config.GetDefaultConfigPath(opts.WorkingDir)
	}
	opts.TerraformPath = filepath.ToSlash(opts.TerraformPath)

	// --- Terragrunt Version
	terragruntVersion, err := hashicorpversion.NewVersion(ctx.App.Version)
	if err != nil {
		// Malformed Terragrunt version; set the version to 0.0
		if terragruntVersion, err = hashicorpversion.NewVersion("0.0"); err != nil {
			return errors.WithStackTrace(err)
		}
	}
	opts.TerragruntVersion = terragruntVersion
	// Log the terragrunt version in debug mode. This helps with debugging issues and ensuring a specific version of terragrunt used.
	opts.Logger.Debugf("Terragrunt Version: %s", opts.TerragruntVersion)

	// --- IncludeModulePrefix
	jsonOutput := false
	for _, arg := range opts.TerraformCliArgs {
		if strings.EqualFold(arg, "-json") {
			jsonOutput = true
			break
		}
	}
	if opts.IncludeModulePrefix && !jsonOutput {
		opts.OutputPrefix = fmt.Sprintf("[%s] ", opts.WorkingDir)
	} else {
		opts.IncludeModulePrefix = false
	}

	// --- Others
	if !opts.RunAllAutoApprove {
		// When running in no-auto-approve mode, set parallelism to 1 so that interactive prompts work.
		opts.Parallelism = 1
	}

	opts.OriginalTerragruntConfigPath = opts.TerragruntConfigPath
	opts.OriginalTerraformCommand = opts.TerraformCommand
	opts.OriginalIAMRoleOptions = opts.IAMRoleOptions

	opts.RunTerragrunt = terraform.Run
	return nil
}

func osExiter(exitCode int) {
	// Do nothing. We just need to override this function, as the default value calls os.Exit, which
	// kills the app (or any automated test) dead in its tracks.
}

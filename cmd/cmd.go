// Package cmd implements the LaunchSafe command-line interface.
//
// Commands:
//
//	launchsafe check   [--path DIR] [--advanced] [--strict] [--format text|json|sarif] [--config FILE]
//	launchsafe version
//	launchsafe help
package cmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rashid1125/launchsafe/internal/checks"
	"github.com/rashid1125/launchsafe/internal/config"
	"github.com/rashid1125/launchsafe/internal/detect"
	"github.com/rashid1125/launchsafe/internal/finding"
	"github.com/rashid1125/launchsafe/internal/output"
)

// Version is set at build time via -ldflags; defaults to "dev".
var Version = "dev"

// ExitCode constants.
const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitUsage   = 2
)

// Run is the main entry point. It returns an exit code.
// Passing in/out/errOut allows tests to capture output.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageText())
		return ExitUsage
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "check":
		return runCheck(rest, stdout, stderr)
	case "version":
		fmt.Fprintf(stdout, "launchsafe version %s\n", Version)
		return ExitSuccess
	case "help", "--help", "-h":
		fmt.Fprintln(stdout, usageText())
		return ExitSuccess
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s\n", cmd, usageText())
		return ExitUsage
	}
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)

	pathFlag := fs.String("path", ".", "project directory to check")
	advancedFlag := fs.Bool("advanced", false, "enable advanced checks")
	strictFlag := fs.Bool("strict", false, "treat warnings as failures (exit 1)")
	formatFlag := fs.String("format", "text", "output format: text, json, sarif")
	configFlag := fs.String("config", "", "path to .launchsafe.json config file")

	if err := fs.Parse(args); err != nil {
		// flag.ContinueOnError returns ErrHelp for -h/--help.
		if err == flag.ErrHelp {
			return ExitSuccess
		}
		return ExitUsage
	}

	// Validate --format.
	format := strings.ToLower(*formatFlag)
	if format != "text" && format != "json" && format != "sarif" {
		fmt.Fprintf(stderr, "invalid --format %q: must be text, json, or sarif\n", *formatFlag)
		return ExitUsage
	}

	// Resolve project path.
	projectPath := *pathFlag
	if !filepath.IsAbs(projectPath) {
		wd, err := os.Getwd()
		if err == nil {
			projectPath = filepath.Join(wd, projectPath)
		}
	}
	if _, err := os.Stat(projectPath); err != nil {
		fmt.Fprintf(stderr, "path %q does not exist or is not accessible: %v\n", projectPath, err)
		return ExitUsage
	}

	// Load config.
	cfg := config.Default()
	if *configFlag != "" {
		loaded, err := config.Load(*configFlag)
		if err != nil {
			fmt.Fprintf(stderr, "config error: %v\n", err)
			return ExitUsage
		}
		cfg = loaded
	} else {
		// Try default config file in the project directory.
		defaultCfg := filepath.Join(projectPath, config.DefaultConfigFile)
		if _, err := os.Stat(defaultCfg); err == nil {
			loaded, err := config.Load(defaultCfg)
			if err != nil {
				fmt.Fprintf(stderr, "config error in %s: %v\n", defaultCfg, err)
				return ExitUsage
			}
			cfg = loaded
		}
	}

	// Detect stacks.
	detection, err := detect.Detect(projectPath)
	if err != nil {
		fmt.Fprintf(stderr, "detection error: %v\n", err)
		return ExitFailure
	}

	// Run checks.
	mode := "basic"
	if *advancedFlag {
		mode = "advanced"
	}
	ctx := checks.Context{
		Root:     projectPath,
		Stacks:   detection.Stacks,
		Advanced: *advancedFlag,
		Config:   cfg,
	}
	findings := checks.Run(ctx)
	sum := finding.Summarize(findings)

	// Build report.
	rep := output.Report{
		Version:  Version,
		Path:     projectPath,
		Mode:     mode,
		Stacks:   detection.Stacks,
		Summary:  sum,
		Findings: findings,
	}

	// Write output.
	var writeErr error
	switch format {
	case "json":
		writeErr = output.WriteJSON(stdout, rep)
	case "sarif":
		writeErr = output.WriteSARIF(stdout, rep)
	default:
		writeErr = output.WriteText(stdout, rep)
	}
	if writeErr != nil {
		fmt.Fprintf(stderr, "output error: %v\n", writeErr)
		return ExitFailure
	}

	// Determine exit code.
	if sum.Fail > 0 {
		return ExitFailure
	}
	if *strictFlag && sum.Warn > 0 {
		return ExitFailure
	}
	return ExitSuccess
}

func usageText() string {
	return strings.TrimSpace(`
Usage:
  launchsafe <command> [flags]

Commands:
  check     Run deployment-readiness checks on a project directory
  version   Print the launchsafe version
  help      Show this help message

Flags for 'check':
  --path DIR       Project directory to check (default: current directory)
  --advanced       Enable advanced checks (lockfiles, secrets scan, framework config, etc.)
  --strict         Treat warnings as failures; return exit code 1
  --format FORMAT  Output format: text (default), json, sarif
  --config FILE    Load rules from a .launchsafe.json config file

Exit codes:
  0  All checks passed (or warnings only, without --strict)
  1  One or more checks failed (or warnings with --strict)
  2  Usage error (invalid command, flag, or config)

Examples:
  launchsafe check
  launchsafe check --path ./myapp --advanced --format json
  launchsafe check --advanced --strict --config .launchsafe.json
  launchsafe version
`)
}

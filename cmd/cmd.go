// Package cmd implements the ShipCheck command-line interface.
//
// Commands:
//
//	shipcheck check   [flags]
//	shipcheck version
//	shipcheck help
package cmd

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rashid1125/shipcheck/internal/checks"
	"github.com/rashid1125/shipcheck/internal/config"
	"github.com/rashid1125/shipcheck/internal/detect"
	"github.com/rashid1125/shipcheck/internal/finding"
	"github.com/rashid1125/shipcheck/internal/output"
)

// Version is set at build time via -ldflags; defaults to "dev".
var Version = "dev"

// ExitCode constants.
const (
	ExitSuccess = 0
	ExitFailure = 1
	ExitUsage   = 2
)

// Run is the main entry point. Returns an exit code.
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
		fmt.Fprintf(stdout, "shipcheck version %s\n", Version)
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
	configFlag := fs.String("config", "", "path to .shipcheck.json config file")
	depthFlag := fs.Int("depth", 0, "scan subdirectories up to N levels deep (0 = top-level only)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return ExitSuccess
		}
		return ExitUsage
	}

	format := strings.ToLower(*formatFlag)
	if format != "text" && format != "json" && format != "sarif" {
		fmt.Fprintf(stderr, "invalid --format %q: must be text, json, or sarif\n", *formatFlag)
		return ExitUsage
	}

	projectPath := *pathFlag
	if !filepath.IsAbs(projectPath) {
		if wd, err := os.Getwd(); err == nil {
			projectPath = filepath.Join(wd, projectPath)
		}
	}
	if _, err := os.Stat(projectPath); err != nil {
		fmt.Fprintf(stderr, "path %q does not exist or is not accessible: %v\n", projectPath, err)
		return ExitUsage
	}

	// Load config: explicit flag > .shipcheck.json > .launchsafe.json (deprecated).
	cfg := config.Default()
	if *configFlag != "" {
		loaded, err := config.Load(*configFlag)
		if err != nil {
			fmt.Fprintf(stderr, "config error: %v\n", err)
			return ExitUsage
		}
		cfg = loaded
	} else {
		newCfg := filepath.Join(projectPath, config.DefaultConfigFile)
		oldCfg := filepath.Join(projectPath, config.DeprecatedConfigFile)
		if _, err := os.Stat(newCfg); err == nil {
			loaded, err := config.Load(newCfg)
			if err != nil {
				fmt.Fprintf(stderr, "config error in %s: %v\n", newCfg, err)
				return ExitUsage
			}
			cfg = loaded
		} else if _, err := os.Stat(oldCfg); err == nil {
			fmt.Fprintf(stderr, "note: .launchsafe.json is deprecated; rename to .shipcheck.json\n")
			loaded, err := config.Load(oldCfg)
			if err != nil {
				fmt.Fprintf(stderr, "config error in %s: %v\n", oldCfg, err)
				return ExitUsage
			}
			cfg = loaded
		}
	}

	mode := "basic"
	if *advancedFlag {
		mode = "advanced"
	}

	var allFindings []finding.Finding
	var allStacks []detect.Stack
	seenStack := map[string]bool{}

	if *depthFlag > 0 {
		// Monorepo mode: scan sub-directories.
		subProjects, err := detect.DetectAll(projectPath, *depthFlag)
		if err != nil {
			fmt.Fprintf(stderr, "detection error: %v\n", err)
			return ExitFailure
		}
		for _, sp := range subProjects {
			spRoot := filepath.Join(projectPath, sp.Path)
			if sp.Path == "." {
				spRoot = projectPath
			}
			ctx := checks.Context{
				Root:     spRoot,
				Stacks:   sp.Stacks,
				Advanced: *advancedFlag,
				Config:   cfg,
			}
			spFindings := checks.Run(ctx)
			for i := range spFindings {
				spFindings[i].SubProject = sp.Path
			}
			allFindings = append(allFindings, spFindings...)
			for _, s := range sp.Stacks {
				key := s.Language + "|" + sp.Path
				if !seenStack[key] {
					seenStack[key] = true
					allStacks = append(allStacks, s)
				}
			}
		}
	} else {
		// Single-project mode.
		detection, err := detect.Detect(projectPath)
		if err != nil {
			fmt.Fprintf(stderr, "detection error: %v\n", err)
			return ExitFailure
		}
		ctx := checks.Context{
			Root:     projectPath,
			Stacks:   detection.Stacks,
			Advanced: *advancedFlag,
			Config:   cfg,
		}
		allFindings = checks.Run(ctx)
		allStacks = detection.Stacks
	}

	sum := finding.Summarize(allFindings)
	rep := output.Report{
		Version:   Version,
		Path:      projectPath,
		Mode:      mode,
		ScanDepth: *depthFlag,
		Stacks:    allStacks,
		Summary:   sum,
		Findings:  allFindings,
	}

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
  shipcheck <command> [flags]

Commands:
  check     Run deployment-readiness checks on a project directory
  version   Print the shipcheck version
  help      Show this help message

Flags for 'check':
  --path DIR       Project directory to check (default: current directory)
  --depth N        Scan subdirectories up to N levels deep (default: 0 = top-level only)
  --advanced       Enable advanced checks (lockfiles, secrets scan, framework config, etc.)
  --strict         Treat warnings as failures; return exit code 1
  --format FORMAT  Output format: text (default), json, sarif
  --config FILE    Load rules from a .shipcheck.json config file

Exit codes:
  0  All checks passed (or warnings only, without --strict)
  1  One or more checks failed (or warnings with --strict)
  2  Usage error (invalid command, flag, or config)

Examples:
  shipcheck check
  shipcheck check --path ./myapp --advanced --format json
  shipcheck check --path ./monorepo --depth 3 --advanced
  shipcheck check --advanced --strict --config .shipcheck.json
  shipcheck version
`)
}

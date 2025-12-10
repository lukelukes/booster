// Package main is the entry point for the booster CLI.
package main

import (
	"booster/internal/cmdexec"
	"booster/internal/condition"
	"booster/internal/config"
	"booster/internal/task"
	"booster/internal/tui"
	"booster/internal/variable"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/alecthomas/kong"
	tea "github.com/charmbracelet/bubbletea"
)

// Build-time variables set via ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// CLI defines the command-line interface.
type CLI struct {
	Config  string     `help:"Path to config file" default:"./bootstrap.yaml" type:"path"`
	Run     RunCmd     `cmd:"" default:"withargs" help:"Run bootstrap tasks (default)"`
	Version VersionCmd `cmd:"" help:"Show version information"`
}

// RunCmd executes bootstrap tasks.
type RunCmd struct {
	DryRun  bool   `help:"Show what would be done without executing"`
	Profile string `help:"Profile to use (required when profiles defined in config)"`
}

// Run executes the bootstrap process.
func (c *RunCmd) Run(cli *CLI) error {
	// Load config
	cfg, err := config.Load(cli.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Validate profile
	profile, err := validateProfile(cfg.Profiles, c.Profile)
	if err != nil {
		return err
	}

	// Resolve variables (if any defined)
	vars, err := resolveVariables(cfg.Variables)
	if err != nil {
		return fmt.Errorf("resolve variables: %w", err)
	}

	// Detect OS and build context with profile
	detector := &condition.SystemDetector{}
	sysCtx := detector.Detect()
	sysCtx.Profile = profile

	// Get config directory for resolving relative paths
	configDir := filepath.Dir(cli.Config)

	// Build tasks first - tasks know their own sudo requirements
	builder := task.DefaultBuilder(sysCtx)
	builder.Register("template.render", task.NewTemplateRenderFactory(task.TemplateRenderConfig{
		Vars:    vars,
		OS:      sysCtx.OS,
		Profile: sysCtx.Profile,
	}))
	builder.Register("pkg-manager.install", task.NewPkgManagerInstallFactory(nil))
	builder.Register("pkg.install", task.NewPkgInstallFactory(task.PkgInstallConfig{
		OS: sysCtx.OS,
	}))
	builder.Register("mise.use", task.NewMiseUseFactory(task.MiseUseConfig{}))
	builder.Register("git.config", task.NewGitConfig(
		cmdexec.DefaultRunner(),
		tui.NewHuhPrompter(),
	))
	builder.Register("set.darwin.defaults", task.NewDarwinDefaultsFactory(task.DarwinDefaultsConfig{
		OS:        sysCtx.OS,
		ConfigDir: configDir,
	}))

	tasks, err := builder.Build(cfg.Tasks)
	if err != nil {
		return fmt.Errorf("build tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks to run")
		return nil
	}

	// Dry-run mode: show what would be done
	if c.DryRun {
		fmt.Printf("Would execute %d task(s):\n\n", len(tasks))
		for i, t := range tasks {
			fmt.Printf("  %d. %s\n", i+1, t.Name())
		}
		return nil
	}

	// Check if any tasks need sudo and prompt if needed (before TUI starts)
	// Tasks self-declare their sudo requirements via NeedsSudo()
	if task.AnyNeedsSudo(tasks) {
		if sudoErr := ensureSudo(); sudoErr != nil {
			return fmt.Errorf("sudo required: %w", sudoErr)
		}
	}

	// Run TUI with mouse support for scrolling
	model := tui.New(tasks)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// resolveVariables converts config variables to definitions and resolves them.
func resolveVariables(cfgVars map[string]config.VariableDef) (map[string]string, error) {
	if len(cfgVars) == 0 {
		return make(map[string]string), nil
	}

	// Convert config definitions to variable definitions
	var defs []variable.Definition
	for name, v := range cfgVars {
		defs = append(defs, variable.Definition{
			Name:    name,
			Prompt:  v.Prompt,
			Default: v.Default,
		})
	}

	// Create store at ~/.local/share/booster/values.yaml
	storePath := defaultValuesPath()
	store := variable.NewFileStore(storePath)

	// Create resolver with TUI collector
	collector := tui.NewPromptCollector()
	resolver := variable.NewResolver(store, variable.WithCollector(collector))

	return resolver.Resolve(defs)
}

// defaultValuesPath returns the default path for storing variable values.
func defaultValuesPath() string {
	// Use XDG_DATA_HOME if set, otherwise ~/.local/share
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if home dir cannot be determined
			home = "."
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "booster", "values.yaml")
}

// VersionCmd shows version information.
type VersionCmd struct{}

// Run prints version information.
func (c *VersionCmd) Run(cli *CLI) error {
	fmt.Printf("booster %s (commit: %s, built: %s)\n", Version, Commit, Date)
	return nil
}

// validateProfile validates the profile flag against configured profiles.
func validateProfile(configured []string, flag string) (string, error) {
	// No profiles in config = no profile filtering needed
	if len(configured) == 0 {
		if flag != "" {
			return "", errors.New("--profile specified but no profiles defined in config")
		}
		return "", nil
	}

	// Profiles defined but no flag = error
	if flag == "" {
		return "", fmt.Errorf("config defines profiles %v, use --profile to select one", configured)
	}

	// Validate flag against configured profiles
	if !slices.Contains(configured, flag) {
		return "", fmt.Errorf("invalid profile %q, must be one of: %v", flag, configured)
	}

	return flag, nil
}

// hasSudoCredentials checks if sudo credentials are already cached.
func hasSudoCredentials() bool {
	// sudo -n true: non-interactive check, succeeds if credentials are cached
	cmd := exec.Command("sudo", "-n", "true")
	return cmd.Run() == nil
}

// ensureSudo prompts for sudo password to cache credentials.
// This runs interactively before the TUI starts.
// Skips prompting if credentials are already cached.
func ensureSudo() error {
	// Check if we already have valid cached credentials
	if hasSudoCredentials() {
		return nil
	}

	fmt.Println("Some tasks require elevated privileges.")
	fmt.Println("Please enter your password to continue...")
	fmt.Println()

	// sudo -v validates and caches credentials (typically for 15 minutes)
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to obtain sudo credentials: %w", err)
	}

	fmt.Println()
	return nil
}

func main() {
	cli := CLI{}
	ctx := kong.Parse(&cli,
		kong.Name("booster"),
		kong.Description("Bootstrap your machine from YAML config"),
		kong.UsageOnError(),
	)

	if err := ctx.Run(&cli); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

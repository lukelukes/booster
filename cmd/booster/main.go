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

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

type CLI struct {
	Config  string     `help:"Path to config file" default:"./bootstrap.yaml" type:"path"`
	Run     RunCmd     `cmd:"" default:"withargs" help:"Run bootstrap tasks (default)"`
	Version VersionCmd `cmd:"" help:"Show version information"`
}

type RunCmd struct {
	DryRun  bool   `help:"Show what would be done without executing"`
	Profile string `help:"Profile to use (required when profiles defined in config)"`
}

func (c *RunCmd) Run(cli *CLI) error {
	cfg, err := config.Load(cli.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	profile, err := validateProfile(cfg.Profiles, c.Profile)
	if err != nil {
		return err
	}

	vars, err := resolveVariables(cfg.Variables)
	if err != nil {
		return fmt.Errorf("resolve variables: %w", err)
	}

	detector := &condition.SystemDetector{}
	sysCtx := detector.Detect()
	sysCtx.Profile = profile

	configDir := filepath.Dir(cli.Config)

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

	if c.DryRun {
		fmt.Printf("Would execute %d task(s):\n\n", len(tasks))
		for i, t := range tasks {
			fmt.Printf("  %d. %s\n", i+1, t.Name())
		}
		return nil
	}

	if task.AnyNeedsSudo(tasks) {
		if sudoErr := ensureSudo(); sudoErr != nil {
			return fmt.Errorf("sudo required: %w", sudoErr)
		}
	}

	model := tui.New(tasks)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func resolveVariables(cfgVars map[string]config.VariableDef) (map[string]string, error) {
	if len(cfgVars) == 0 {
		return make(map[string]string), nil
	}

	var defs []variable.Definition
	for name, v := range cfgVars {
		defs = append(defs, variable.Definition{
			Name:    name,
			Prompt:  v.Prompt,
			Default: v.Default,
		})
	}

	storePath := defaultValuesPath()
	store := variable.NewFileStore(storePath)

	collector := tui.NewPromptCollector()
	resolver := variable.NewResolver(store, variable.WithCollector(collector))

	return resolver.Resolve(defs)
}

func defaultValuesPath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "booster", "values.yaml")
}

type VersionCmd struct{}

func (c *VersionCmd) Run(cli *CLI) error {
	fmt.Printf("booster %s (commit: %s, built: %s)\n", Version, Commit, Date)
	return nil
}

func validateProfile(configured []string, flag string) (string, error) {
	if len(configured) == 0 {
		if flag != "" {
			return "", errors.New("--profile specified but no profiles defined in config")
		}
		return "", nil
	}

	if flag == "" {
		return "", fmt.Errorf("config defines profiles %v, use --profile to select one", configured)
	}

	if !slices.Contains(configured, flag) {
		return "", fmt.Errorf("invalid profile %q, must be one of: %v", flag, configured)
	}

	return flag, nil
}

func hasSudoCredentials() bool {
	cmd := exec.Command("sudo", "-n", "true")
	return cmd.Run() == nil
}

func ensureSudo() error {
	if hasSudoCredentials() {
		return nil
	}

	fmt.Println("Some tasks require elevated privileges.")
	fmt.Println("Please enter your password to continue...")
	fmt.Println()

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

package main

import (
	"booster/internal/action"
	"booster/internal/config"
	"booster/internal/expr"
	"booster/internal/run"
	"booster/internal/ui"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var errChangesPending = errors.New("changes pending")

type PlanCmd struct {
	Profile string `help:"Profile to use (required when profiles defined in config)"`
}

func (c *PlanCmd) Run(cli *CLI) error {
	ticker := time.NewTicker(run.SudoRefreshInterval)
	defer ticker.Stop()
	return c.run(context.Background(), cli, newTerminal(cli, c.Profile, ticker.C))
}

func (c *PlanCmd) run(ctx context.Context, cli *CLI, term terminal) error {
	plan, err := buildPlan(ctx, cli, c.Profile)
	if err != nil {
		return err
	}

	view := plan.View()
	review := term.review
	if !view.HasChanges() {
		review = showPlan(term.out, false)
	}
	applied, err := reviewAndApply(ctx, plan, term, review, ui.ModeDrift)
	if err != nil {
		return err
	}
	if !applied && view.HasChanges() {
		return errChangesPending
	}
	return nil
}

func buildPlan(ctx context.Context, cli *CLI, profileFlag string) (run.PlanResult, error) {
	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	plan, err := planSteps(ctx, cli, profileFlag)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return run.PlanResult{}, fmt.Errorf("plan interrupted: %w", ctxErr)
	}
	return plan, err
}

func planSteps(ctx context.Context, cli *CLI, profileFlag string) (run.PlanResult, error) {
	cfg, err := config.Load(cli.Config)
	if err != nil {
		return run.PlanResult{}, fmt.Errorf("load config: %w", err)
	}

	profile, err := validateProfile(cfg.Profiles, profileFlag)
	if err != nil {
		return run.PlanResult{}, err
	}

	vars, err := resolveVariables(cfg.Variables)
	if err != nil {
		return run.PlanResult{}, fmt.Errorf("resolve variables: %w", err)
	}

	varsAny := make(map[string]any, len(vars))
	for k, v := range vars {
		varsAny[k] = v
	}
	exprCtx := expr.NewContext().WithProfile(profile).WithVars(varsAny).WithConfigDir(filepath.Dir(cli.Config))

	compiled, err := run.Compile(cfg.Tasks, exprCtx, action.Registry())
	if err != nil {
		return run.PlanResult{}, fmt.Errorf("compile: %w", err)
	}

	plan, err := run.Plan(ctx, compiled)
	if err != nil {
		return run.PlanResult{}, fmt.Errorf("plan: %w", err)
	}

	return plan, nil
}

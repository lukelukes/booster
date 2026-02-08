# Design of Booster

Booster is a declarative machine bootstrap tool. You write a YAML file describing what your machine should look like, and Booster makes it so. This document explains why it works the way it does -- the choices made, the alternatives rejected, and the trade-offs accepted.


## Why Go

Before discussing what Booster does, it is worth discussing why it is written in Go, because the implementation language shapes the user experience in ways that are easy to overlook.

The most important property of Go for Booster is that it compiles to a single static binary. A bootstrap tool has a chicken-and-egg problem: it needs to be installable on a machine that does not yet have its development environment set up. If Booster were written in Python, you would need Python installed first. If it were written in Rust, you would need a Rust toolchain (or a pre-compiled binary, which Rust also supports well). Go's compilation model means Booster is a single file with no runtime dependencies. Download it, run it. No package managers, no runtimes, no shared libraries.

Go's standard library is also unusually well-suited to system tool development. File operations, path manipulation, template rendering, process execution, YAML parsing (via a well-established third-party library) -- all of these are available without pulling in a large dependency tree. The `os/exec` package handles the subprocess management that Booster needs for package installation and tool invocation. The `text/template` package provides the template engine used by the `template.render` action. These are not incidental conveniences; they are the core operations of a bootstrap tool, and having them built into the language ecosystem reduces the surface area for dependency-related issues.

Go's concurrency primitives (goroutines and channels) power the TUI's event loop through Bubble Tea, but Booster's task execution is deliberately sequential. The concurrency is used for presentation -- streaming log output while the UI remains responsive -- not for parallel task execution. This is a good example of using a language's strengths where they help without being tempted into using them everywhere.

Cross-compilation is another practical consideration. Go can produce binaries for Linux (amd64, arm64) and macOS (amd64, arm64) from a single build machine. This is not unique to Go -- Rust does the same -- but Go makes it trivially easy with `GOOS` and `GOARCH` environment variables. For a tool that needs to support two operating systems and two architectures (Intel and Apple Silicon Macs, x86 and ARM Linux), this means four binaries from one build command.

The compilation speed matters too, though for the developer rather than the user. Go compiles a full project in seconds, which keeps the edit-compile-test cycle fast. This is less important for the end user (who downloads a pre-built binary) but significant for development velocity, especially during the kind of rapid iteration that happens when debugging platform-specific behavior on multiple machines.

The trade-off is that Go is more verbose than languages like Python or Ruby, and its error handling is famously repetitive. But for a tool whose codebase is modest in size and whose operations are mostly I/O-bound, these are minor costs. The benefits -- fast compilation, easy cross-compilation for Linux and macOS, a single deployable artifact, strong standard library -- outweigh them.

Go's type system also shapes the architecture in positive ways. The `Task` interface is a simple contract: `Name() string`, `Run(ctx context.Context) Result`, `NeedsSudo() bool`. Any struct that implements these three methods is a task. There is no inheritance hierarchy, no abstract base class, no framework to buy into. The `PackageManager` interface abstracts over Homebrew and pacman with the same simplicity. This interface-oriented design keeps the codebase flat and navigable -- you can understand any action by reading a single file, without tracing up an inheritance chain.

The testing story also benefits from Go's design. Interfaces make dependency injection trivial: the `cmdexec.Runner` interface abstracts over process execution, allowing tests to use a mock runner that returns predetermined output without actually executing system commands. Package installation tests can verify that `PkgInstall` correctly identifies missing packages and calls the right install commands without touching a real package manager. Symlink tests can verify creation logic without creating real filesystem entries. This is standard Go testing practice, but it aligns well with Booster's needs -- a bootstrap tool must be tested without bootstrapping a real machine.

This testability concern is not academic. A tool that modifies system state (installing packages, creating files, changing configuration) is notoriously difficult to test in isolation. The mock-based approach means that unit tests exercise the decision logic (which packages are missing, whether a symlink already exists, whether a template output has changed) without the side effects. Integration tests, which do execute against real system state, are run separately and less frequently. This two-tier testing strategy gives fast feedback on logic changes and confidence on integration behavior.


## The Problem with Shell Scripts

Most developers bootstrap new machines with shell scripts. It starts simple: a `setup.sh` that installs a few packages and symlinks some dotfiles. Then it grows. You add a conditional for macOS vs Linux. Then another for Arch vs Ubuntu. You wrap things in functions. You add flags. You handle errors -- sometimes. A year later, the script is 400 lines of fragile bash that nobody wants to touch, and it broke on the last OS reinstall because `brew` changed its default path.

Shell scripts are imperative. They describe steps, not outcomes. This makes them brittle in a fundamental way: they encode assumptions about the current state of the system at every line. "Install git" fails if git is already installed (or doesn't, depending on the package manager, and whether you passed `--needed`, and whether you remembered to). "Create this directory" fails if it exists (or doesn't, depending on whether you used `mkdir` or `mkdir -p`, and whether you remembered). Every line is a potential point of failure that requires defensive coding, and most people don't write defensive bash.

The deeper problem is idempotency. A good bootstrap script should be safe to run multiple times. If you ran it yesterday and run it again today, the second run should be a no-op -- or at least not break anything. This is extraordinarily difficult to achieve in shell. Every command needs a guard: check if the package is installed before installing, check if the symlink exists before creating it, check if the git config is already set before setting it. These guards are half the script, and they're the half most likely to have bugs.

Booster takes a different approach. You declare what you want, and Booster figures out what to do about it. If a package is already installed, it skips it. If a symlink already points to the right place, it moves on. If a directory exists, nothing happens. Idempotency is built into every action, not bolted on by the user.

This is not a new idea. Declarative configuration management has existed since CFEngine in the 1990s. What is relatively new is applying it to the personal machine bootstrap problem, which has historically been considered too small and too varied to justify a dedicated tool. The argument for Booster is that "too small" is exactly where shell scripts cause the most pain -- because nobody invests in making them robust for something they use twice a year.

There is also a readability argument. A `bootstrap.yaml` file is self-documenting in a way that a shell script is not. The YAML config declares "these are the packages I want, these are the symlinks I want, these are the git settings I want." A shell script mixes these declarations with the implementation details of checking, installing, and error handling. Six months from now, when you are setting up a new machine and want to add a package to your bootstrap config, finding the right place in a YAML file is trivial. Finding the right place in a 400-line bash script -- and being confident that you are not breaking something -- is not.

The declarative approach also makes the configuration reviewable in a way that imperative scripts are not. You can look at a Booster config and immediately see what packages will be installed, what symlinks will be created, what git config will be set. There is no control flow to trace, no variable state to track, no conditional execution paths to enumerate. The config is a snapshot of desired state, and understanding it requires only reading, not executing it in your head.

This reviewability extends to sharing configurations. A developer can look at a colleague's `bootstrap.yaml` and quickly understand their setup -- what tools they use, how they organize their dotfiles, what macOS preferences they change. With a shell script, the same understanding requires reading through conditional branches, function definitions, and variable expansions. The YAML config is closer to a readable specification than to executable code, and that readability is a form of documentation that requires no additional effort to maintain.

The combination of declarative config with `when` guards does introduce some complexity, though. A config with many conditional tasks -- different packages per OS, different symlinks per profile, different tools per architecture -- can become difficult to mentally "execute" for a specific platform/profile combination. You have to scan through the list, evaluate each `when` clause in your head, and assemble the effective task list. This is the cost of using composition (actions + conditions) rather than separate config files per platform. Booster's dry-run mode mitigates this by showing the effective task list for a given platform and profile without executing anything. In practice, most configs have a handful of conditional tasks among many unconditional ones, and the mental overhead is manageable.


## Why YAML

YAML is not a beloved format. It has well-documented quirks (the Norway problem, implicit type coercion, significant whitespace). The choice to use it anyway was pragmatic.

The alternatives are JSON, TOML, and custom DSLs. JSON lacks comments and is verbose for human authoring. TOML handles flat configuration well but becomes awkward for nested structures like task lists with varying argument shapes. A custom DSL would need its own parser, its own documentation, its own editor support -- a significant investment for a tool whose configuration language is not its core value proposition.

YAML works because Booster configs are structurally simple. A config file is a flat list of tasks, each with an action name and some arguments. The nesting is shallow. The types are basic: strings, lists of strings, and small key-value maps. In this narrow domain, YAML's readability advantages outweigh its quirks. A task that installs packages looks like what it does:

```yaml
- action: pkg.install
  args:
    - git
    - ripgrep
    - neovim
```

There is no configuration format that would make this meaningfully clearer.

The version field (`version: "1"`) exists because YAML configs tend to evolve. When (not if) the schema changes in a breaking way, the version field allows Booster to give clear error messages rather than cryptic parse failures. It is a small upfront cost that prevents a common class of frustration.

One decision worth noting is the structure of the config itself. It is intentionally flat: a list of tasks at the top level, not a hierarchy of groups or categories. There are no sections, no nesting of tasks within tasks, no grouping by purpose. This flatness is unusual -- most configuration tools organize by category (packages, files, services) -- but it serves the sequential execution model well. The config reads top to bottom, and execution proceeds top to bottom. There is no ambiguity about what runs when.

The flat structure also avoids a common configuration design trap: the question of how deeply to nest. If you have categories, do subcategories make sense? If packages are one category, are Homebrew casks a subcategory? Are AUR packages? This kind of hierarchical design leads to endless debates about taxonomy that provide no functional benefit. In Booster, every task is just a task. The action field says what it does. The `when` field says whether it should. The flat list says when. There is nothing else to decide.

The task schema itself is deliberately minimal: `action`, `args`, and optionally `when`. There is no `name` or `description` field for human-readable annotations (though task identity via an `id` field is being introduced for technical reasons -- see [Where This Is Going](#where-this-is-going)). Each action generates its own display name from its arguments: a package install shows the package names, a symlink shows the source and target paths, a git config shows the keys. This keeps the config DRY: you do not need to write "Install development tools" as a description for a task whose arguments already say `[neovim, git, ripgrep]`.

The `args` field is intentionally polymorphic -- its shape varies by action. `dir.create` takes a list of strings. `symlink.create` takes a list of `{source, target}` maps. `pkg.install` takes a mixed list of strings and maps. `set.darwin.defaults` takes a map with a `file` key pointing to an external YAML file. This polymorphism is the most YAML-like aspect of Booster's config: the shape of data depends on context. The benefit is that each action's args look natural for what they represent. The cost is that understanding the args format requires knowing the action, which means users need to consult the [actions reference](../reference/actions.md). This is an acceptable trade-off for a tool with eight actions.


## The Bootstrap Niche

Booster exists in a space already occupied by several established tools. Understanding why it exists requires understanding what those tools actually do and where they leave gaps.

**chezmoi** is excellent at dotfile management. It tracks your configuration files in a git repository, supports templating, handles secrets, and can apply your dotfiles to a new machine. But dotfiles are only part of bootstrapping. chezmoi does not install packages, create directories, set git configuration, configure macOS defaults, or install development tool versions. You still need something else for the rest.

**Ansible** can do all of this and more. It is a general-purpose automation framework with thousands of modules covering everything from package installation to cloud infrastructure provisioning. The problem is weight. Ansible requires Python, has a complex dependency tree, uses an inventory/playbook model designed for managing fleets of servers, and has a learning curve calibrated for DevOps engineers managing production infrastructure. Using Ansible to bootstrap a personal laptop is like using a forklift to move a couch: it works, but the overhead is absurd.

**Nix and NixOS** represent the most principled approach to declarative system configuration. NixOS can describe an entire operating system declaratively, with rollbacks and reproducibility guarantees that nothing else matches. The trade-off is totality: Nix demands that you buy into its entire ecosystem. You don't just add Nix to your workflow; you replace your workflow with Nix. The learning curve is steep, the documentation assumes familiarity with functional programming concepts, and the Nix language is unlike anything most developers have encountered. For someone who just wants to install their packages and symlink their dotfiles after a fresh OS install, Nix is a philosophical commitment that may not be warranted.

**GNU Stow** handles symlink management specifically -- creating symlinks from a structured directory tree. It is elegant and minimal but handles only one part of the bootstrap problem. **Make** is sometimes repurposed as a bootstrap runner, with targets for different setup steps, but Makefiles are imperative, not idempotent by default, and the syntax is hostile to newcomers. Various **dotfile managers** (yadm, dotbot, rcm) overlap with parts of Booster's functionality but are focused on dotfiles specifically rather than the broader bootstrap problem.

The pattern across these alternatives is that each excels in a specific dimension but leaves gaps in others. Booster's position is that machine bootstrapping is a coherent problem that deserves a coherent solution -- not a patchwork of specialized tools stitched together with shell scripts.

It is worth acknowledging that the "right" choice among these tools is personal and contextual. A NixOS enthusiast who has already invested in learning Nix has no reason to use Booster -- Nix is strictly more powerful. A developer who only needs dotfile management and is happy with a shell script for the rest might find chezmoi sufficient. Booster is for the developer who wants more structure than shell scripts provide, less commitment than Nix demands, and broader scope than dotfile managers offer. That is a real niche, but it is a niche, and Booster does not pretend otherwise.

Booster fills a specific gap: the one-shot bootstrap for an individual's machine. Install packages, create directories, symlink dotfiles, render templates, set git config, configure macOS preferences. Do it once after a fresh install, and maybe again six months later when you get a new laptop. The scope is deliberately narrow. Booster is not a configuration management system, not a continuous state enforcement tool, not infrastructure as code. It runs when you tell it to and does nothing in between.

The "individual" part of this positioning is important. Booster is not designed for teams deploying fleets of developer workstations. It has no centralized configuration server, no role-based access, no audit logging. These features are essential for enterprise tooling but add complexity that is pointless for someone setting up their own machine. The design assumes a single user who both authors and executes the configuration -- the same person who will debug any issues.

This narrowness is a feature. It means the tool can be simple -- a single binary with no runtime dependencies beyond the system's own package manager. It means the configuration format can be minimal. It means the execution model can be straightforward. Every piece of complexity that Booster does not take on is complexity that its users do not need to understand.

There is a reasonable counter-argument: why not just use chezmoi for dotfiles and a short shell script for the rest? Many developers do exactly this, and it works fine. The case for Booster is that "the rest" is not as simple as it appears. Package installation, directory creation, template rendering, git configuration, macOS defaults -- these are individually trivial but collectively messy when done imperatively. The value of Booster is not any single action but the unified declarative model across all of them. One config file, one execution, one tool. Whether that unification justifies a dedicated tool is a judgment call that depends on how many machines you set up and how much you value reproducibility. Booster's bet is that enough developers care to make it worthwhile.


## Expressions Over Arbitrary Shell

A common pattern in YAML-based tools is embedding shell commands for conditional logic:

```yaml
when: "[ $(uname) = 'Darwin' ]"
run: "apt-get install -y {{ packages | join(' ') }}"
```

This is convenient and powerful. It is also a maintenance and security hazard.

Shell embedded in YAML is difficult to test, difficult to validate statically, and prone to subtle portability issues between shells and platforms. It reintroduces exactly the fragility that a declarative approach is supposed to eliminate. The moment you allow arbitrary shell in your configuration, you have a shell script wearing a YAML costume.

There is also a portability dimension. macOS ships with zsh as the default shell; many Linux users use bash, fish, or zsh. Shell syntax varies across these -- a bashism in a `when` clause silently breaks on a stock macOS machine. Even `[` vs `[[`, `==` vs `=`, and quoting rules differ between shells. A bootstrap tool must work on a fresh OS install, which means it must work with whatever shell is default, and that default varies. By avoiding embedded shell entirely, Booster eliminates this entire class of problems.

Booster uses [expr-lang](https://expr-lang.org/) for conditional logic and value interpolation. Expressions use `${ ... }` delimiters and operate in a sandboxed environment with a fixed set of context variables and functions:

```yaml
when: ${ os == "darwin" }
when: ${ profile == "work" and installed("docker") }
args:
  - ${ home }/.config/nvim
```

The expression language supports comparison, logical operators, list membership, string operations, and a small set of built-in functions (`exists`, `installed`, `which`, `expand`, and a few others). It does not support arbitrary function calls, file system writes, network access, or shell execution.

This is a deliberate constraint. The expression system observes system state but cannot modify it. It can check whether a file exists, whether a program is installed, what the current OS is -- but it cannot create files, install programs, or change system state. Mutation is the exclusive domain of actions, which are explicit, logged, visible in the TUI, and wrapped in the execution framework.

The trade-off is real. There are things you cannot express with Booster's expression language that you could express with shell. If you need to parse the output of a command, check a network endpoint, or perform complex string manipulation, Booster's expressions will not cover it. The bet is that these cases are uncommon enough in machine bootstrapping that the safety and predictability of a constrained expression language is worth the reduced flexibility. In practice, the vast majority of bootstrap conditionals are about platform detection (`os == "darwin"`), profile selection (`profile == "work"`), and tool availability (`installed("mise")`), all of which the expression system handles cleanly.

There is a deeper principle at work here: **expressions observe, actions mutate**. This separation is fundamental to Booster's design. The expression system is pure -- it reads system state but never changes it. All mutations happen through actions, which are explicit, logged, and wrapped in the execution framework. This means a dry-run mode can evaluate every expression and show exactly what would happen without risking side effects. It means expressions are safe to evaluate multiple times, in any order, without worrying about one expression's side effects affecting another. It means the entire conditional logic layer is predictable in a way that embedded shell fundamentally cannot be.

The choice of [expr-lang](https://expr-lang.org/) specifically (rather than writing a custom expression evaluator) reflects the same pragmatism as choosing YAML. Expr-lang is a Go library with a well-defined semantics, good performance, and no external dependencies. It supports the operators and types that Booster needs, and it can be extended with custom functions (which is how `exists`, `installed`, and `which` are provided). Writing a custom expression evaluator would have meant reimplementing parsing, type checking, and evaluation -- engineering effort better spent on the bootstrap logic itself.

The expression system has two modes of operation that are worth distinguishing. A *full expression* is a string that is entirely a `${ ... }` block and returns the expression's native type -- bool for `when` guards, string or list for args. An *interpolated expression* embeds `${ ... }` blocks within a larger string and always resolves to a string. The path `${ home }/.config/nvim` is an interpolation: it splices the value of `home` into a string literal. The guard `${ os == "darwin" }` is a full expression: it evaluates to a boolean.

This distinction matters because it determines what types flow through the system. Full expressions preserve types: a `when` guard must evaluate to bool, and a type error is caught at evaluation time. Interpolations are always strings, because they are string templates by nature. This is a pragmatic choice that avoids the complexity of a full type system while still catching the most common class of errors (non-boolean `when` values).

The built-in functions are all read-only probes: `exists` checks if a path exists, `installed` checks if a binary is in PATH, `which` returns the path of a binary, `expand` resolves `~` and environment variables in paths, `default` provides fallback values for potentially empty variables. These functions cannot fail in dangerous ways -- the worst case is a false negative (reporting that a file does not exist when it does, due to a permissions issue). They are designed to be safe for use in expressions that evaluate before any task executes, which means they must not assume anything about system state that a prior task might have changed. This constraint is another aspect of the expression-observe-action-mutate separation.

The function set is deliberately small. Adding functions is a commitment: every function becomes part of the API that users depend on and that must be maintained. A larger function library would provide more convenience but also more surface area for bugs and more documentation to maintain. The current set covers the common patterns in bootstrap configurations -- path checking, tool detection, value defaulting -- and additional functions can be added when concrete use cases justify them, not speculatively.


## Sequential Execution

Booster executes tasks in the order they appear in the configuration file, one at a time. There is no dependency graph, no topological sort, no parallelism.

This is the simplest possible execution model, and simplicity matters here for several reasons.

First, bootstrap is infrequent. You run Booster once after an OS install, maybe once more when you get a new machine. The difference between a 2-minute sequential run and a 45-second parallel run is irrelevant when the tool runs twice a year. Optimizing for execution speed in a tool used this rarely is premature optimization in its purest form.

Second, sequential execution has a trivial mental model. Task 5 runs after task 4 and before task 6. If task 5 depends on something task 3 installed, it works, because task 3 already ran. Users do not need to declare dependencies, do not need to reason about execution order, do not need to debug race conditions. The configuration file is a recipe, read top to bottom. When something goes wrong, the user can look at the config, find the failed task, and understand exactly what ran before it and what did not run after it. There is no need to trace a dependency graph to understand the execution path.

Third, parallelism introduces complexity that propagates through the entire system. Error handling becomes harder: if three parallel tasks fail simultaneously, which error do you show? Partial failure recovery becomes harder: if task A succeeded and task B failed in a parallel group, do you retry just B? The TUI becomes harder: how do you show progress for multiple simultaneous tasks? Each of these problems has solutions, but every solution adds complexity, and the benefit (faster bootstrap runs for something that runs twice a year) does not justify the cost.

The sequential model also means that task ordering is the user's responsibility, which is both a burden and a form of control. If you need to install `mise` before using `mise.use` to install Go, you put the `mise` installation task first. This is explicit and obvious, unlike a dependency system where you declare that task B "needs" task A and hope the resolver gets the order right.

There is also a debugging advantage to sequential execution. When something goes wrong, the user knows exactly what happened: every task before the failure ran to completion (or was skipped), and no task after the failure has started. There are no concurrent side effects to reason about, no partially completed parallel groups, no question about whether task A's failure was caused by task B running simultaneously. The execution trace is a simple sequence, and debugging a sequence is straightforward.

DAG-based execution is a tool for systems that run frequently and have complex interdependencies -- CI pipelines, build systems, infrastructure provisioners. Booster is none of these things.

One criticism of sequential execution is that it makes failure recovery cumbersome. If task 30 of 50 fails, the user must fix the issue and re-run from the beginning. Tasks 1-29 will be skipped (because they are idempotent and their desired state is already present), but the user still sees them scroll by. This is a real annoyance, and solutions like `--start-from` or `--retry-failed` flags are being considered. But the underlying observation is important: the annoyance is cosmetic, not functional. Thanks to idempotency, the re-run is always safe. The skipped tasks take negligible time. The UX could be better, but the correctness is never in question.

There is a subtlety in the sequential model that is worth calling out: the distinction between static and runtime task visibility. Some tasks can be determined to be irrelevant before execution starts -- a `when: ${ os == "darwin" }` task on a Linux machine will never run, and this is known at parse time. Other tasks depend on runtime state -- a task guarded by `when: ${ installed("docker") }` cannot be evaluated until execution time, because a prior task might install Docker. Booster is evolving toward making this distinction explicit: statically irrelevant tasks would not appear in the execution plan at all, while runtime-skipped tasks would appear as "skipped" with a reason. This gives users a cleaner view of what is happening without changing the fundamental sequential model.


## Primitives Over Features

Booster's design philosophy is to provide composable building blocks rather than special-purpose features. This is not a novel idea -- it is the Unix philosophy applied to a configuration tool -- but it has specific implications for how Booster handles user requests.

When someone says "I want to install packages conditionally based on the OS," the feature-oriented response is to add a `platform` field to the package install action:

```yaml
- action: pkg.install
  platform: darwin
  args: [coreutils, gnu-sed]
```

The primitive-oriented response is to observe that this is just the combination of two existing primitives -- a conditional expression and a package install action:

```yaml
- action: pkg.install
  when: ${ os == "darwin" }
  args:
    - coreutils
    - gnu-sed
```

The second approach does not require any new code. It composes existing pieces. And it generalizes: the same `when` primitive works for any action, not just package installation. You can conditionally create directories, conditionally render templates, conditionally set git config -- all with the same mechanism.

This philosophy shapes what Booster does not build as much as what it does. There is no `tags` system for grouping tasks, because profiles combined with `when` expressions cover the same use cases. There is no `retry` mechanism per task, because the idempotent execution model means re-running the entire config is safe. There is no `include` directive for splitting configs across files, because a single file is simpler and sufficient for the typical bootstrap config (which rarely exceeds 100 tasks).

The risk of this philosophy is austerity. Primitives require users to assemble solutions themselves, which is more work than using a purpose-built feature. If the primitives are not expressive enough, users hit walls. But in the bootstrap domain, the surface area of user needs is well-bounded: install things, create things, link things, configure things, and do so conditionally based on platform and preference. A small set of primitives covers this space.

The concrete primitives Booster provides are:

- **Actions** -- the verbs: create directories, install packages, create symlinks, render templates, set git config, configure macOS defaults, install tool versions via mise.
- **Expressions** -- the conditionals and value computation: platform detection, profile matching, tool availability checks, path interpolation.
- **Profiles** -- the selection mechanism: "work" vs "personal", chosen at runtime.
- **Variables** -- the user input: values collected interactively and persisted between runs.

Profiles are worth a closer look. A profile is simply a string selected at runtime (via the `--profile` flag or an interactive prompt). It has no special behavior of its own -- it is just a value that expressions can reference. The config declares available profiles (`profiles: [personal, work]`), and tasks use `when: ${ profile == "work" }` to conditionally execute. There is no profile inheritance, no profile composition, no profile-specific variable overrides. A profile is a label, nothing more.

This minimal design might seem limiting. Other tools support profile hierarchies, per-profile variable files, and profile-conditional includes. But consider the use cases: a developer has a work laptop and a personal laptop. The work laptop needs a VPN client, a corporate git config, and access to internal package repositories. The personal laptop does not. A single string -- "work" or "personal" -- is sufficient to distinguish these cases. The `when` expression system handles the rest. If someone has three profiles that share 90% of their tasks and differ by 10%, the shared tasks have no `when` clause (they always run), and the differing tasks have profile-specific guards. This is explicit and readable, if slightly verbose. The verbosity is the price of not having a profile inheritance system, and it is a price worth paying for the simplicity of "a profile is a string."

These four categories compose to handle the full range of bootstrap scenarios. Conditional package installation is `pkg.install` + `when`. Profile-specific dotfiles are `symlink.create` + `when` on profile. Templated configuration with user-specific values is `template.render` + variables. No special features required -- just primitives combined.

This composability is the test for whether a feature request should be fulfilled by a new primitive or by combining existing ones. "I want to install different packages on macOS and Linux" does not need a new `platform` field on `pkg.install` -- it needs two task entries with different `when` clauses. "I want to set git config only on my work machine" does not need a `profile` field on `git.config` -- it needs a `when: ${ profile == "work" }`. The question is always: can the existing primitives express this? If yes, the answer is composition, not a new feature. If no, the answer might be a new primitive -- but it should be general enough to solve multiple problems, not just the one that prompted the request.

There is an honest tension here. Composition requires the user to know what primitives are available and how they combine. A purpose-built `platform` field on `pkg.install` would be more discoverable than `when: ${ os == "darwin" }` for someone reading the docs for the first time. Booster leans toward composability over discoverability because the primitive set is small enough to learn in an afternoon, and the payoff -- fewer concepts, more flexibility -- compounds over time. But this is a trade-off, not an obvious win.


## The Action Vocabulary

Booster ships with a fixed set of actions. You cannot define custom actions, write plugins, or extend the action vocabulary without modifying the source code. This is another deliberate constraint.

The reasoning is domain closure. Machine bootstrapping -- for an individual developer on Linux or macOS -- involves a known set of operations: installing packages, managing files, configuring tools. The eight actions Booster provides (dir.create, symlink.create, template.render, pkg.install, pkg-manager.install, mise.use, git.config, set.darwin.defaults) cover this domain. If an action is missing, it likely means the tool's scope should be expanded, not that users should work around it with plugins.

A plugin system would add significant complexity: a plugin API to design and maintain, a loading mechanism, security considerations (plugins run with the user's privileges), version compatibility between plugins and the host, and the inevitable ecosystem fragmentation where critical functionality migrates from the core into third-party plugins of varying quality. For a tool whose scope is deliberately narrow, this complexity is not justified.

Every action in Booster is idempotent by design. `pkg.install` checks what is already installed before installing. `symlink.create` verifies the existing link target. `template.render` compares the rendered output against the existing file. `dir.create` checks if the directory exists. This idempotency is possible precisely because the action set is fixed and each action is purpose-built by the maintainers with idempotency as a requirement. A plugin system would push this burden to plugin authors, and experience with other ecosystems suggests it would not be reliably met.

The actions are also platform-aware in ways that would be difficult to achieve with plugins. `pkg.install` automatically selects the right package manager: Homebrew on macOS, an AUR helper (paru by default) on Arch Linux. `set.darwin.defaults` is a no-op on Linux. This platform awareness is baked into each action and tested against the supported platforms.

Each action follows a consistent pattern internally: check current state, compare against desired state, take action only if they differ, report what happened. This pattern is the source of idempotency, and it is enforced by convention and testing rather than by a framework. There is no `IdempotentAction` base class or trait that actions must implement. Instead, each action implements the `Task` interface (which requires `Name`, `Run`, and `NeedsSudo` methods) and is responsible for its own state checking. This is simpler than a framework-enforced approach, at the cost of requiring discipline from action authors. Given that the action set is small and maintained by the same team, this trade-off favors simplicity.

The factory pattern deserves mention here. Each action type has a factory function that takes raw YAML arguments (as `any`) and produces concrete task instances. A single config entry for `pkg.install` with ten packages produces a single `PkgInstall` task that installs all ten. A single entry for `symlink.create` with five source-target pairs produces five `SymlinkCreate` tasks, one per link. This fan-out is handled by the factory, not by the user. The user writes what they want; the factory figures out the granularity. This keeps the config concise while giving the executor and TUI per-operation visibility.

The different fan-out behaviors across actions are deliberate. Package installation fans one task entry into one executable task (because package managers handle batch installation efficiently). Symlink creation fans one entry into many tasks (because each symlink is an independent operation that can succeed or fail individually, and the user wants to see per-link status). Template rendering fans similarly to symlinks. This means the TUI task count does not correspond one-to-one to config entries, which can be surprising but is correct -- you want to see that 4 of 5 symlinks succeeded and 1 failed, not that "the symlink task partially failed."

The use of `any` as the factory input type is a deliberate trade-off. Go does not have sum types or discriminated unions, so the alternatives are either `any` with runtime type assertions or a family of strongly-typed arg structs. The `any` approach was chosen because it allows the YAML unmarshaling to remain simple (unmarshal into `map[string]any` and let each factory validate its own args). The validation happens in the factory, not in the config parser, which keeps the parser generic and pushes domain knowledge into the action implementations where it belongs. The cost is that arg validation errors appear at factory invocation time rather than at unmarshal time, and error messages require care to be helpful.


## Interactive TUI

Booster uses a terminal user interface (TUI) as its primary execution interface. This is unusual for a tool that is essentially a batch processor, and it deserves explanation.

Machine bootstrapping is inherently interactive. You need to choose a profile (work laptop vs personal machine). You need to provide values that cannot be hardcoded (your git email, your preferred editor). You need to see what is happening -- which packages are being installed, which tasks are being skipped, whether something has gone wrong. And you need this feedback in real time, because some tasks (compiling a package from source, downloading large casks) take minutes.

A TUI provides all of this in a single interface. The task list shows overall progress. The log panel shows real-time output from the current task. The summary screen shows what happened. Variable prompts and profile selection are integrated into the flow.

The alternative is traditional CLI output: lines of text streaming to the terminal. This works, but it is difficult to navigate. When installing 40 packages, the output from package 3 scrolls off the screen by the time package 40 finishes. Finding the error from a failed task means scrolling through potentially hundreds of lines of output from successful ones. A TUI gives structure to this information: you can navigate to any task and see its specific output.

The trade-off is that TUIs do not work in non-interactive contexts. You cannot pipe Booster's output to a file, you cannot run it in a headless CI environment, you cannot script it. This is an intentional limitation. Booster is for individuals sitting at their terminal, bootstrapping their machine. It is not for automation, not for CI, not for headless deployment. The TUI is a natural fit for this use case, and the use cases it does not serve are explicitly out of scope.

Importantly, the TUI is a presentation layer only. It does not contain business logic. The executor, the task implementations, the expression evaluation, the variable resolution -- all of this happens in packages that know nothing about the TUI. The TUI observes execution through messages and renders the state. This separation means the core logic is testable without a terminal, and a future non-TUI mode (for validation or debugging) would not require rearchitecting the system.

The TUI is built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), a Go framework based on the Elm architecture: a model holds state, an update function processes messages and returns a new model, and a view function renders the model to the screen. This architecture enforces the separation between logic and presentation at the framework level. The TUI never calls task methods directly; it sends commands and receives messages. The coordinator component bridges between task execution and TUI updates, ensuring that log output and task results flow through the message system rather than being coupled directly.

This architecture has a practical benefit beyond code organization: the TUI is testable. Because the view is a pure function of the model, and the model is updated through discrete messages, test code can send messages and assert on the resulting view output without needing an actual terminal. The TUI tests verify layout behavior, progress rendering, and failure display without spawning a pseudo-terminal.

The choice to use a TUI rather than a web UI or native GUI is also about deployment pragmatics. A TUI needs only a terminal emulator, which every target user already has. A web UI would need a local server, a browser, and a way to communicate between them. A native GUI would need platform-specific windowing code. The TUI is the lightest-weight interactive interface that still provides structured navigation, real-time updates, and keyboard-driven interaction.

There is also a cultural argument. Booster's target users are developers who are comfortable in the terminal. They use vim keybindings (Booster supports `j`/`k` for navigation), they expect keyboard-driven interaction, and they prefer tools that integrate with their existing terminal workflow rather than opening a separate window. The TUI respects this by being a tool that lives where its users already are.

The dry-run mode is worth mentioning in this context. When invoked with `--dry-run`, Booster evaluates all conditions and shows what would happen without executing any actions. The TUI displays the task list as it would appear during a real run, but no actions are performed. This relies on the expression-observe-action-mutate separation: because expressions are side-effect-free, the entire conditional evaluation can run safely in dry-run mode. The result is a preview that is as accurate as possible without actually modifying the system -- the only inaccuracies come from runtime guards that depend on prior task outputs (which do not exist in a dry run).


## Variables and User Input

Variables are Booster's mechanism for values that cannot be known at config authoring time. Your git email, your preferred editor, your work domain -- these vary between users and sometimes between machines. Hardcoding them in the config defeats the purpose of a reusable bootstrap configuration.

Booster handles variables through a resolution chain with three levels. First, environment variables are checked: if a variable named `GitEmail` is defined and `$GitEmail` is set in the environment, the environment value wins. Second, previously stored values are checked: Booster persists variable values to a YAML file (`~/.local/share/cli/values.yaml` by default) so they survive between runs. Third, if neither source has a value, Booster prompts the user interactively.

This resolution order is deliberate. Environment variables take priority because they allow override without modifying stored state -- useful for one-off runs or testing. Stored values take priority over prompting because the common case is re-running a config where you have already provided your values, and being re-prompted for every variable on every run would be maddening. Prompting is the fallback for genuinely new values.

The persistence of variable values between runs is what makes Booster's idempotency practical for interactive data. Without persistence, every re-run would require the user to re-enter all their variable values. With persistence, a re-run on an already-bootstrapped machine skips prompts (because values are stored) and skips actions (because the desired state already exists), making the entire run effectively a no-op -- which is exactly what idempotent execution should look like.

The storage location (`~/.local/share/cli/values.yaml`) follows the XDG Base Directory Specification, which is the standard for user data storage on Linux. The values file is plain YAML, human-readable and editable. If a user wants to change a stored value, they can edit the file directly rather than re-running Booster. This is intentional: the persistence layer should not be a black box. It is a file you can read, edit, back up, and version-control like any other configuration.

One deliberate omission in the variable system is secret handling. Variables are stored in plain text on disk. If you need to store a secret (an API key, a token, a password), the variables file is not the right place. This is a conscious choice: secret management is a complex problem with its own tools (1Password CLI, Bitwarden CLI, system keychains), and Booster does not attempt to solve it. For values that are secret, the recommended approach is to use environment variables (which take priority in the resolution chain) or to handle them outside of Booster entirely.

Variables are available in expressions as `vars.Name`, which means they can appear in `when` guards, in action arguments, and in template rendering. A variable defined once can be used in a git.config action, a template, and a conditional guard -- another instance of the primitives-over-features philosophy, where a single mechanism serves multiple purposes.

There is a question of scope: should variables support types beyond strings? Should they support computed values, validation rules, or dependencies on other variables? The current answer is no. Variables are strings, collected by prompt, with an optional default. This is the simplest model that works for the bootstrap use case, where variable values are things like email addresses, usernames, and paths -- all naturally strings. Adding types and validation would make the variable system more robust but also more complex, and the failure modes it would prevent (entering an invalid email in a git.config prompt) are better caught by the action itself than by the variable system. This may change, but for now, simplicity wins.

The interaction between variables and templates is worth noting. The `template.render` action receives all resolved variables in a `TemplateContext`, along with system information (OS, profile). Templates use Go's `text/template` syntax, which is different from Booster's `${ ... }` expression syntax. This is a genuine point of confusion: `${ vars.GitEmail }` in a YAML arg is a Booster expression, while `{{ .Vars.GitEmail }}` in a template file is a Go template directive. The two systems coexist because they serve different purposes -- Booster expressions operate on the config at parse/compile time, while Go templates operate on file content at render time -- but the syntactic difference is an acknowledged rough edge.

An alternative design would be to use the same expression syntax everywhere -- Booster expressions in both config args and template files. This was considered and rejected because Go's `text/template` is significantly more powerful for file generation (it supports loops, conditionals, pipelines, and custom functions within the template itself), and replacing it with Booster's expression syntax would sacrifice that power for syntactic consistency. The template system needs to handle complex file generation; the expression system needs to handle simple value interpolation and boolean conditions. They are different tools for different jobs, and unifying them would weaken both.

The `git.config` action has an interesting interaction with variables that illustrates how primitives compose. Git config items can specify either a static `value` or a `prompt` string. When a prompt is specified and the key has no existing value, Booster prompts the user at task execution time. This is separate from the global variable system -- it is an action-specific prompt for a value that is consumed immediately by git config rather than stored in the variables file. This dual-prompt design (global variables for reusable values, action-specific prompts for one-off values) avoids polluting the variable namespace with values that are only relevant to a single action.


## Platform Support

Booster supports Linux and macOS. Windows is explicitly not supported, and this is not a "not yet" -- it is a "not planned."

The bootstrap workflow Booster targets is rooted in Unix conventions: dotfiles in the home directory, symlinks for configuration management, shell environments, package managers that work from the command line. Windows has different conventions (the registry, different paths, different file system semantics), and supporting it would require significant changes to nearly every action. `symlink.create` would need to handle Windows symlink permissions. `pkg.install` would need a Windows package manager (Chocolatey, Scoop, winget). Path handling would need to account for drive letters and backslashes. `set.darwin.defaults` is macOS-only by definition.

More fundamentally, the developer workflows that Booster targets are predominantly Unix-based. WSL exists for developers who use Windows but want Unix tooling, and Booster works in WSL because WSL is Linux.

Within the supported platforms, Booster handles macOS and Linux differently where needed. Package installation uses Homebrew on macOS and pacman/paru on Arch Linux. The expression context exposes the OS as a distro identifier on Linux (`"arch"`, `"ubuntu"`, `"fedora"`) rather than the generic `"linux"`, because the interesting conditional logic on Linux is about which distro you are on, not that you are on Linux. macOS is identified as `"darwin"`.

This asymmetry is practical rather than elegant. On macOS, there is effectively one way to manage packages (Homebrew). On Linux, the landscape is fragmented. Currently, Booster has first-class support for Arch Linux via pacman/AUR helpers. Support for other distributions is less developed. This reflects the maintainers' actual usage rather than an attempt at universality. The expression system provides an escape hatch: if a user is on Fedora, they can write `when: ${ os == "fedora" }` to guard tasks, even if the built-in package management actions do not yet support dnf.

A reasonable question is: why not abstract over package managers more aggressively? Ansible does this with its `package` module, which dispatches to the appropriate backend. The answer is that package names are not portable. The package called `fd` on Arch is `fd-find` on Ubuntu. The package called `ripgrep` on both is `rg` on neither -- but the binary it installs is called `rg`. Abstracting over the package manager installation command is straightforward; abstracting over package naming across distributions is a bottomless pit. Booster sidesteps this by letting users write distribution-specific task blocks with `when` guards, which is more verbose but always correct.

The `set.darwin.defaults` action is an interesting case study in platform-specific design. macOS has a unique system preferences mechanism (the `defaults` command) that has no Linux equivalent. Rather than trying to abstract "system preferences" across platforms, Booster provides a macOS-specific action that is simply a no-op on Linux. This is honest: macOS preferences and Linux system configuration are fundamentally different, and pretending otherwise would produce a leaky abstraction. The `when` expression system provides a clean way to guard platform-specific tasks, so there is no need for the action itself to handle cross-platform logic.

The `mise.use` action is another interesting case. [mise](https://mise.jdx.dev/) is a development tool version manager, and Booster delegates to it rather than reimplementing version management. This is a deliberate choice to compose with existing tools rather than replace them. Booster installs mise (via `pkg.install` or `pkg-manager.install`), then uses mise to install specific tool versions. The boundary is clear: Booster handles the one-time setup of "mise is installed and configured for these tools," and mise handles the ongoing management of tool versions. This is the same "first 30 minutes" philosophy applied to tools within the bootstrap.

Similarly, Homebrew casks (graphical macOS applications distributed through Homebrew) are a macOS-only concept. The `pkg.install` action supports a `casks` field that is only valid on macOS. On Linux, specifying casks is an error. This is explicit about platform differences rather than silently ignoring them, which is better for users who might be sharing a config between machines.

The `pkg-manager.install` action exists because of another bootstrapping chicken-and-egg problem. On Arch Linux, users often want an AUR helper like paru or yay, but these helpers are not in the official repositories -- they must be built from source. On macOS, Homebrew itself might not be installed on a fresh machine. The `pkg-manager.install` action handles these bootstrap-the-bootstrapper scenarios, installing the package manager that subsequent `pkg.install` tasks will use. This is a narrowly targeted action that solves a specific sequencing problem, and its existence is another argument for the fixed action vocabulary: this is the kind of platform-specific knowledge that belongs in the tool, not in user scripts.


## What Booster Is Not

Understanding a tool's design means understanding its boundaries. Booster is not:

**A configuration management system.** It does not continuously enforce state. It does not run on a schedule. It does not detect drift. It runs when you invoke it and does nothing between invocations. If you manually change a setting after bootstrapping, Booster will not revert it (though running Booster again would reset it, since actions are idempotent).

**An automation tool.** It requires a human at the keyboard for profile selection, variable input, and monitoring. There is no headless mode, no API, no webhook integration. This is intentional -- the interactive nature is a feature, not a limitation to be worked around.

**A dotfile manager.** It can create symlinks to your dotfiles and render templates, but it does not track dotfile changes, does not integrate with version control, and does not handle secrets. If you need sophisticated dotfile management, use chezmoi alongside Booster.

**A package manager.** It delegates to the system's package manager. It does not resolve dependencies, does not manage repositories, does not handle package conflicts. It tells the package manager what to install and trusts it to do the right thing.

**A general-purpose scripting tool.** It does not execute arbitrary commands as a primary mechanism. The absence of a generic `run` or `shell` action is intentional: the moment you can run arbitrary commands, every other design constraint becomes advisory rather than enforceable. Idempotency becomes the user's problem. Safety becomes the user's problem. The expression system becomes unnecessary because you can just use shell conditionals. A `run` action would be the thin end of a wedge that turns Booster back into a shell script with extra steps.

**A continuous state manager.** Tools like Puppet, Chef, and Ansible are designed to run periodically and converge system state. They detect drift -- "someone manually changed this file since the last run" -- and correct it. Booster has no daemon, no agent, no scheduled execution. If a user modifies a symlink that Booster created, Booster will not notice until the next manual run. This is appropriate for a bootstrap tool: you set up the machine once, and after that, you are in control. The machine does not need to be continuously nudged back into shape, because you are the only user and you know what you changed.

These boundaries are load-bearing. Every time Booster declines to be something, it avoids the complexity that comes with being that thing. The result is a tool that does one thing well: getting a fresh machine from "just installed the OS" to "ready to work."

There is a philosophical dimension to these boundaries. Many tools start narrow and grow broad over time, accumulating features in response to user requests until they become sprawling, complex systems. Booster's explicit non-goals are a commitment to resist this trajectory. Not every user request should be fulfilled; some should be met with "that is outside Booster's scope, and here is a tool that does it well." chezmoi for dotfile versioning, mise for tool version management, the system's package manager for package updates. Booster orchestrates the initial setup but does not try to subsume the tools it invokes.

This is harder than it sounds. When users ask for a feature, the path of least resistance is to add it. Saying "no, use another tool for that" requires confidence that the scope boundary is in the right place and discipline to maintain it. The test Booster applies is: does this feature relate to the one-time bootstrap of a fresh machine? If yes, it might belong. If it relates to ongoing system maintenance, it does not. Package installation is bootstrap. Package updates are maintenance. Symlink creation is bootstrap. Symlink synchronization with a git repository is maintenance (that is chezmoi's job). Template rendering for initial configuration is bootstrap. Watching files for changes and re-rendering is maintenance. This line is not always crisp, but having a clear principle to apply makes the decision process more consistent than ad hoc feature evaluation.

The useful mental model is: Booster is the first 30 minutes after an OS install. It gets you from "blank machine" to "my machine." After that, other tools take over -- your editor manages its own plugins, your shell manages its own config, your package manager manages its own updates. Booster does not need to do any of these things because it does not need to be running after the first 30 minutes are over.


## Error Handling and Failure

How a tool handles failure reveals its priorities. Booster's approach to errors is guided by two principles: fail early when possible, and provide enough context to fix the problem.

Validation errors -- malformed YAML, unknown action names, invalid expression syntax -- should surface before any task executes. It is far worse to discover a syntax error at task 30 (after 29 tasks have already modified the system) than to discover it before anything runs. The planned compile step formalizes this: all config validation happens during compilation, and execution only begins if the config is valid.

Runtime errors -- a package installation that fails, a source file that does not exist for a symlink, a git command that returns an error -- stop the current task but do not necessarily stop execution. The executor records the failure, the TUI shows the error, and subsequent tasks continue. This is a pragmatic choice: a failure to install one package should not prevent the rest of the bootstrap from running. The user can see what failed, fix the underlying issue, and re-run (which, thanks to idempotency, will skip everything that already succeeded and retry only what failed).

There is no automatic retry mechanism. Retrying failed package installations or network-dependent operations seems like a natural feature, but it introduces questions (how many times? with what backoff? only for certain error types?) that are better answered by the user than by policy. If an installation fails, the user can investigate, fix the issue, and re-run. The cost of a manual re-run is low because of idempotency; the cost of a misguided automatic retry (silently retrying something that will never succeed, masking a real problem) is higher.

The TUI's per-task log display is part of the error handling story. When a task fails, the user can navigate to that task and see its full output -- the package manager's error message, the missing file path, the git error. This context is preserved even after execution continues to later tasks. The alternative -- dumping all output to a scrollback buffer -- makes it difficult to find the relevant error in the noise of successful operations.

A design decision that follows from this is the choice to continue execution after a failure rather than stopping immediately. The reasoning: on a fresh machine, most tasks are independent. A failure to install one package does not prevent creating directories or setting up symlinks. Stopping at the first failure would leave the machine in a more incomplete state than necessary. The user can assess the failures in the summary view and address them, knowing that everything else that could succeed did succeed.

This does not apply to tasks with implicit dependencies. If installing mise fails, the subsequent `mise.use` tasks will also fail. Booster does not model these dependencies explicitly -- it relies on the user ordering tasks sensibly and on actions failing gracefully when prerequisites are missing (the `mise.use` action checks for mise in PATH and fails with a clear message if it is not found). This is another area where the simplicity of the sequential model trades explicit dependency declarations for implicit ordering assumptions.

The `NeedsSudo` method on the `Task` interface reflects another aspect of error handling philosophy. Some actions (package installation on Linux) require elevated privileges, while others (creating symlinks, rendering templates) do not. Rather than running the entire tool as root or requesting sudo upfront, Booster tracks which tasks need elevation and can inform the user accordingly. This is about transparency: the user should know which operations require elevated privileges before they run, not discover it when a task fails with a permissions error.

Note that `NeedsSudo` is platform-dependent. Package installation on macOS (via Homebrew) does not require sudo, while package installation on Linux (via pacman/paru) typically does. Homebrew was specifically designed to avoid requiring root privileges, while the pacman ecosystem expects them. This platform difference is encoded in the action implementation rather than in the config, keeping the config platform-agnostic while the actions handle platform specifics.


## Idempotency as a Core Guarantee

Idempotency deserves its own discussion because it is not just a nice property -- it is the foundation that makes everything else work. The promise of Booster is: run it as many times as you want, and the result is always the same desired state.

This guarantee flows from a design rule: every action must check before it acts. `pkg.install` queries the package manager for already-installed packages and only installs the missing ones. `symlink.create` reads the existing link target and skips creation if it already points to the right source. `template.render` renders the template to memory, compares the result with the existing file byte-for-byte, and only writes if they differ. `dir.create` stats the path and skips if the directory exists. `git.config` reads the current value and skips if it matches.

This check-then-act pattern has a subtle but important property: it means Booster's skip messages are meaningful. When the TUI shows "skipped: already exists" for a symlink task, it is not just skipping the task for performance -- it is confirming that the desired state is already present. The skip is an assertion that passes, not work that was avoided. This makes a Booster re-run function as a verification step: run it after bootstrapping, and the all-skipped result confirms that your machine is in the expected state.

The check-then-act pattern is also not atomic. There is a window between "check that the file does not exist" and "create the file" where another process could intervene. Booster does not use file locking or atomic operations for most actions. This is acceptable because bootstrap runs are interactive, supervised operations where the user is watching. It would not be acceptable for a continuous state enforcement system, which is another reason Booster does not try to be one.

There is a nuance in how different actions report their results. Each task execution returns a `Result` with a status (done, skipped, failed, pending) and a message. The status is machine-readable; the message is human-readable. "Skipped" for `pkg.install` means "all packages already installed." "Skipped" for `symlink.create` means "link already points to the correct target." "Skipped" for `git.config` means "all keys already have the desired values." The uniformity of the status (always one of four values) allows the executor and TUI to handle all actions generically, while the per-action message provides the specificity users need to understand what happened.

This three-status model (done, skipped, failed -- pending is transient) is minimal by design. There is no "warning" status, no "partial success," no "changed vs unchanged" distinction beyond skipped-vs-done. This simplicity is possible because actions are designed to be all-or-nothing: either the desired state was achieved (done), the desired state was already present (skipped), or something went wrong (failed). Partial success -- "installed 3 of 5 packages" -- is reported as a failure with a message describing what happened. This keeps the mental model simple: green means good, yellow means nothing to do, red means something broke.

The absence of a "changed" status (as distinct from "done") is a conscious omission. Ansible, for example, distinguishes between "ok" (desired state was already present) and "changed" (the action modified something). This distinction is useful for continuous management -- you want to know when your system drifts -- but less useful for bootstrap, where you generally want everything to be "done" on the first run and "skipped" on subsequent runs. Adding a "changed" status would double the cognitive load of interpreting results for minimal benefit in the bootstrap context.


## Where This Is Going

Booster is in active development, and some design decisions are still settling. The expression system is being refined to be the sole mechanism for conditional logic, replacing an earlier condition system. A compile step is being introduced between configuration parsing and execution, which will enable stricter validation and clearer separation between what can be determined statically and what requires runtime evaluation.

The direction is toward more rigor, not more features. Stricter validation (malformed expressions should fail loudly, not degrade to literals). Clearer semantics (the difference between a task that was never applicable and a task that was skipped at runtime). Better composability (expressions that can reference the output of prior tasks, enabling runtime decisions based on system probing).

What is not on the roadmap is equally important: no plugin system, no parallel execution, no dependency graphs, no CI mode, no Windows support. These are not missing features waiting to be added. They are complexity that has been evaluated and rejected because it does not serve the core use case: an individual developer, sitting at their terminal, setting up their machine.

The compile step mentioned above deserves elaboration, because it represents a significant architectural evolution. Currently, Booster moves almost directly from YAML parsing to task execution. The planned compile step introduces an intermediate representation -- a `Plan` -- that validates the configuration, resolves static conditions, and produces a clean execution list before any task runs. This matters for two reasons. First, it means all validation errors surface before execution begins, rather than discovering a typo in task 30 after tasks 1-29 have already run. Second, it enables the static-vs-runtime distinction for conditions: tasks with statically false guards are omitted from the plan entirely, while tasks with runtime-dependent guards are included but may be skipped during execution. The user sees only the tasks that are relevant to their machine, with runtime skips clearly labeled.

This is the kind of change that does not add features visible to users but makes the tool fundamentally more reliable. It is also the kind of change that is only possible because Booster is still in its alpha stage, where breaking changes are acceptable. Doing it now avoids the compatibility burden of doing it later.

The notion of task identity is closely related to the compile step. Currently, tasks are anonymous: they are identified by position in the list. The introduction of required task IDs enables two things. First, it allows expressions to reference prior task outputs (`tasks["install-docker"].status`), enabling runtime decisions based on what actually happened during execution. Second, it allows validation of task references at compile time: referencing a task that does not exist, or referencing a task that comes later in the execution order, can be caught before any task runs. This is a small schema change with large implications for the safety of the configuration model.

The broader arc of Booster's development is one of progressive formalization. It started as a relatively direct YAML-to-execution pipeline and is evolving toward a more disciplined architecture with clear phase boundaries (parse, compile, execute) and explicit semantics for each phase. The goal is not to add power -- the current action set is sufficient for the bootstrap domain -- but to make the existing power more predictable, more validatable, and more transparent to users.

Each phase has a clear responsibility. Parsing converts YAML text into a config structure. Compilation validates the config, resolves static conditions, evaluates compile-time expressions, expands factories, and produces a plan. Execution runs the plan sequentially, recording results and evaluating runtime conditions. This separation means that errors from each phase have distinct character: parse errors are about syntax, compile errors are about semantics (unknown actions, invalid expressions, bad references), and execution errors are about system state (package not found, permission denied, network unavailable). Users can reason about and address these different error types differently.

A tool that runs on a fresh OS install needs to be trustworthy above all else. You do not want surprises when you are setting up a machine you need to use today. Every design decision in Booster -- the declarative model, the constrained expression language, the sequential execution, the fixed action vocabulary, the interactive TUI, the deliberate scope limitations -- serves this goal. Trustworthiness is not a feature; it is the cumulative result of choosing simplicity, predictability, and transparency at every level.

---

See also:
- [Config reference](../reference/config.md) for the configuration file structure
- [Expressions reference](../reference/expressions.md) for the expression system details
- [Actions reference](../reference/actions.md) for available actions and their arguments

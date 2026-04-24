package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	configpkg "bytemind/internal/config"
	extensionspkg "bytemind/internal/extensions"
	extensionsruntimepkg "bytemind/internal/extensionsruntime"
)

func RunExt(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	_ = stdin
	if len(args) == 0 {
		renderExtUsage(stdout)
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "-h", "--help":
		renderExtUsage(stdout)
		return nil
	case "list":
		return runExtList(args[1:], stdout, stderr)
	case "load":
		return runExtLoad(args[1:], stdout, stderr)
	case "unload":
		return runExtUnload(args[1:], stdout, stderr)
	case "status", "show":
		return runExtStatus(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown ext subcommand %q", args[0])
	}
}

func runExtList(args []string, stdout, stderr io.Writer) error {
	workspace, configPath, err := parseExtCommonFlags("ext list", args, stderr)
	if err != nil {
		return err
	}
	manager, err := newCLIExtensionManager(workspace, configPath)
	if err != nil {
		return err
	}
	items, listErr := manager.List(context.Background())
	renderExtStatuses(stdout, items)
	return listErr
}

func runExtLoad(args []string, stdout, stderr io.Writer) error {
	workspace, configPath, source, err := parseExtActionTarget("ext load", args, stderr)
	if err != nil {
		return err
	}
	manager, err := newCLIExtensionManager(workspace, configPath)
	if err != nil {
		return err
	}
	info, err := manager.Load(context.Background(), source)
	if err != nil {
		return err
	}
	renderExtDetail(stdout, info)
	return nil
}

func runExtUnload(args []string, stdout, stderr io.Writer) error {
	workspace, configPath, extensionID, err := parseExtActionTarget("ext unload", args, stderr)
	if err != nil {
		return err
	}
	manager, err := newCLIExtensionManager(workspace, configPath)
	if err != nil {
		return err
	}
	if err := manager.Unload(context.Background(), extensionID); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "unloaded extension %s\n", extensionID)
	return nil
}

func runExtStatus(args []string, stdout, stderr io.Writer) error {
	workspace, configPath, extensionID, err := parseExtActionTarget("ext status", args, stderr)
	if err != nil {
		return err
	}
	manager, err := newCLIExtensionManager(workspace, configPath)
	if err != nil {
		return err
	}
	info, err := manager.Get(context.Background(), extensionID)
	if err != nil {
		return err
	}
	renderExtDetail(stdout, info)
	return nil
}

func newCLIExtensionManager(workspace, configPath string) (extensionspkg.Manager, error) {
	cfg, err := configpkg.Load(workspace, configPath)
	if err != nil {
		return nil, err
	}
	base := extensionspkg.NewManager(workspace)
	return extensionsruntimepkg.NewManager(workspace, configPath, base, cfg), nil
}

func parseExtActionTarget(name string, args []string, stderr io.Writer) (workspace string, configPath string, target string, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspaceFlag := fs.String("workspace", "", "Workspace directory; defaults to current directory")
	configFlag := fs.String("config", "", "Path to config file")
	target = ""
	if len(args) > 0 && !strings.HasPrefix(strings.TrimSpace(args[0]), "-") {
		target = strings.TrimSpace(args[0])
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return "", "", "", err
	}
	if target == "" && fs.NArg() > 0 {
		target = strings.TrimSpace(fs.Arg(0))
	}
	if target == "" {
		return "", "", "", fmt.Errorf("usage: bytemind %s <source-or-extension-id>", name)
	}
	workspace, err = ResolveWorkspace(*workspaceFlag)
	if err != nil {
		return "", "", "", err
	}
	return workspace, strings.TrimSpace(*configFlag), target, nil
}

func parseExtCommonFlags(name string, args []string, stderr io.Writer) (workspace string, configPath string, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspaceFlag := fs.String("workspace", "", "Workspace directory; defaults to current directory")
	configFlag := fs.String("config", "", "Path to config file")
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	workspace, err = ResolveWorkspace(*workspaceFlag)
	if err != nil {
		return "", "", err
	}
	return workspace, strings.TrimSpace(*configFlag), nil
}

func renderExtUsage(w io.Writer) {
	lines := []string{
		"bytemind ext list [--workspace path] [--config path]",
		"bytemind ext load <source> [--workspace path] [--config path]",
		"bytemind ext unload <extension-id> [--workspace path] [--config path]",
		"bytemind ext status <extension-id> [--workspace path] [--config path]",
	}
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

func renderExtStatuses(w io.Writer, items []extensionspkg.ExtensionInfo) {
	if len(items) == 0 {
		fmt.Fprintln(w, "no extensions discovered")
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	fmt.Fprintf(w, "%-22s %-8s %-9s %-5s %s\n", "ID", "KIND", "STATUS", "TOOL", "MESSAGE")
	for _, item := range items {
		fmt.Fprintf(
			w,
			"%-22s %-8s %-9s %-5d %s\n",
			item.ID,
			item.Kind,
			item.Status,
			item.Capabilities.Tools,
			firstNonEmptyValue(strings.TrimSpace(item.Health.Message), "-"),
		)
	}
}

func renderExtDetail(w io.Writer, item extensionspkg.ExtensionInfo) {
	fmt.Fprintf(w, "id: %s\n", item.ID)
	fmt.Fprintf(w, "name: %s\n", firstNonEmptyValue(strings.TrimSpace(item.Name), item.ID))
	fmt.Fprintf(w, "kind: %s\n", item.Kind)
	fmt.Fprintf(w, "status: %s\n", item.Status)
	fmt.Fprintf(w, "source_scope: %s\n", item.Source.Scope)
	fmt.Fprintf(w, "source_ref: %s\n", firstNonEmptyValue(strings.TrimSpace(item.Source.Ref), "-"))
	fmt.Fprintf(w, "tools: %d\n", item.Capabilities.Tools)
	fmt.Fprintf(w, "message: %s\n", firstNonEmptyValue(strings.TrimSpace(item.Health.Message), "-"))
	fmt.Fprintf(w, "last_error: %s\n", firstNonEmptyValue(strings.TrimSpace(string(item.Health.LastError)), "-"))
	fmt.Fprintf(w, "checked_at: %s\n", firstNonEmptyValue(strings.TrimSpace(item.Health.CheckedAtUTC), "-"))
}

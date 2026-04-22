package tools

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type systemSandboxPolicy struct {
	FileIsolation    bool
	ProcessIsolation bool
	NetworkIsolation bool
}

type systemSandboxLaunchSpec struct {
	ArgPrefix []string
	Policy    systemSandboxPolicy
}

type systemSandboxPlatformBackend interface {
	Name() string
	Probe(lookPath func(string) (string, error)) (string, error)
	ShellLaunchSpec() systemSandboxLaunchSpec
	WorkerLaunchSpec() systemSandboxLaunchSpec
}

type linuxUnshareSystemSandboxBackend struct{}

func (linuxUnshareSystemSandboxBackend) Name() string {
	return "linux_unshare"
}

func (linuxUnshareSystemSandboxBackend) Probe(lookPath func(string) (string, error)) (string, error) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	runner, err := lookPath("unshare")
	if err != nil || strings.TrimSpace(runner) == "" {
		return "", errors.New("linux backend \"unshare\" is unavailable")
	}
	return runner, nil
}

func (linuxUnshareSystemSandboxBackend) ShellLaunchSpec() systemSandboxLaunchSpec {
	return systemSandboxLaunchSpec{
		ArgPrefix: append(linuxSystemSandboxNamespaceArgs(), "sh", "-lc"),
		Policy: systemSandboxPolicy{
			FileIsolation:    true,
			ProcessIsolation: true,
			NetworkIsolation: true,
		},
	}
}

func (linuxUnshareSystemSandboxBackend) WorkerLaunchSpec() systemSandboxLaunchSpec {
	return systemSandboxLaunchSpec{
		ArgPrefix: append([]string(nil), linuxSystemSandboxWorkerArgs()...),
		Policy: systemSandboxPolicy{
			FileIsolation:    true,
			ProcessIsolation: true,
			NetworkIsolation: false,
		},
	}
}

type systemSandboxRuntimeBackend struct {
	Enabled           bool
	Name              string
	Runner            string
	Shell             systemSandboxLaunchSpec
	Worker            systemSandboxLaunchSpec
	UnavailableReason string
}

func resolveSystemSandboxRuntimeBackend(mode, goos string, lookPath func(string) (string, error)) (systemSandboxRuntimeBackend, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == systemSandboxModeOff {
		return systemSandboxRuntimeBackend{}, nil
	}

	backend := systemSandboxBackendForOS(goos)
	if backend == nil {
		if mode == systemSandboxModeRequired {
			return systemSandboxRuntimeBackend{}, fmt.Errorf("system sandbox mode required but no backend is available on %s", goos)
		}
		return systemSandboxRuntimeBackend{
			UnavailableReason: fmt.Sprintf("no backend is available on %s", strings.TrimSpace(goos)),
		}, nil
	}

	runner, err := backend.Probe(lookPath)
	if err != nil || strings.TrimSpace(runner) == "" {
		if mode == systemSandboxModeRequired {
			if err != nil {
				return systemSandboxRuntimeBackend{}, fmt.Errorf("system sandbox mode required but %s", err.Error())
			}
			return systemSandboxRuntimeBackend{}, errors.New("system sandbox mode required but backend is unavailable")
		}
		reason := "backend is unavailable"
		if err != nil {
			reason = strings.TrimSpace(err.Error())
		}
		if reason == "" {
			reason = "backend probe returned an empty runner path"
		}
		return systemSandboxRuntimeBackend{
			Name:              backend.Name(),
			UnavailableReason: reason,
		}, nil
	}

	return systemSandboxRuntimeBackend{
		Enabled: true,
		Name:    backend.Name(),
		Runner:  runner,
		Shell:   backend.ShellLaunchSpec(),
		Worker:  backend.WorkerLaunchSpec(),
	}, nil
}

func systemSandboxBackendForOS(goos string) systemSandboxPlatformBackend {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "linux":
		return linuxUnshareSystemSandboxBackend{}
	case "darwin":
		return darwinSandboxExecSystemSandboxBackend{}
	case "windows":
		return windowsJobObjectSystemSandboxBackend{}
	default:
		return nil
	}
}

type darwinSandboxExecSystemSandboxBackend struct{}

func (darwinSandboxExecSystemSandboxBackend) Name() string {
	return "darwin_sandbox_exec"
}

func (darwinSandboxExecSystemSandboxBackend) Probe(lookPath func(string) (string, error)) (string, error) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	runner, err := lookPath("sandbox-exec")
	if err != nil || strings.TrimSpace(runner) == "" {
		return "", errors.New("darwin backend \"sandbox-exec\" is unavailable")
	}
	return runner, nil
}

func (darwinSandboxExecSystemSandboxBackend) ShellLaunchSpec() systemSandboxLaunchSpec {
	return systemSandboxLaunchSpec{
		Policy: systemSandboxPolicy{
			FileIsolation:    true,
			ProcessIsolation: true,
			NetworkIsolation: false,
		},
	}
}

func (darwinSandboxExecSystemSandboxBackend) WorkerLaunchSpec() systemSandboxLaunchSpec {
	return systemSandboxLaunchSpec{
		Policy: systemSandboxPolicy{
			FileIsolation:    true,
			ProcessIsolation: true,
			NetworkIsolation: false,
		},
	}
}

type windowsJobObjectSystemSandboxBackend struct{}

func (windowsJobObjectSystemSandboxBackend) Name() string {
	return "windows_job_object"
}

func (windowsJobObjectSystemSandboxBackend) Probe(func(string) (string, error)) (string, error) {
	return "windows-job-object", nil
}

func (windowsJobObjectSystemSandboxBackend) ShellLaunchSpec() systemSandboxLaunchSpec {
	return systemSandboxLaunchSpec{
		Policy: systemSandboxPolicy{
			FileIsolation:    false,
			ProcessIsolation: true,
			NetworkIsolation: false,
		},
	}
}

func (windowsJobObjectSystemSandboxBackend) WorkerLaunchSpec() systemSandboxLaunchSpec {
	return systemSandboxLaunchSpec{
		Policy: systemSandboxPolicy{
			FileIsolation:    false,
			ProcessIsolation: true,
			NetworkIsolation: false,
		},
	}
}

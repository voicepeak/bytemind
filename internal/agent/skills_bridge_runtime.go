package agent

import (
	"errors"
	"strings"

	extensionspkg "bytemind/internal/extensions"
	toolspkg "bytemind/internal/tools"
)

var nonBridgedBuiltinTools = map[string]struct{}{
	"list_files":      {},
	"read_file":       {},
	"search_text":     {},
	"web_search":      {},
	"web_fetch":       {},
	"write_file":      {},
	"replace_in_file": {},
	"apply_patch":     {},
	"update_plan":     {},
	"run_shell":       {},
}

func (r *Runner) syncActiveSkillBridges(active *activeSkillRuntime) error {
	if r == nil {
		return nil
	}
	registry, ok := r.registry.(*toolspkg.Registry)
	if !ok || registry == nil {
		return nil
	}

	r.bridgeMu.Lock()
	defer r.bridgeMu.Unlock()

	if active == nil {
		r.clearActiveSkillBridgesLocked(registry)
		return nil
	}

	extensionID := activeSkillExtensionID(active)
	if extensionID == "" {
		r.clearActiveSkillBridgesLocked(registry)
		return nil
	}
	if r.activeSkillBridgeID != extensionID {
		r.clearActiveSkillBridgesLocked(registry)
		r.activeSkillBridgeID = extensionID
	}
	if r.activeSkillBridgeToolKeys == nil {
		r.activeSkillBridgeToolKeys = map[string]struct{}{}
	}

	desired := map[string]struct{}{}
	for _, item := range active.Skill.ToolPolicy.Items {
		original := normalizeBridgePolicyItem(item)
		if original == "" || !shouldBridgePolicyItem(original) {
			continue
		}
		resolved, exists := registry.Get(original)
		if !exists {
			continue
		}
		binding, err := extensionspkg.RegisterBridgedToolWithOptions(registry, extensionspkg.ExtensionTool{
			Source:      extensionspkg.ExtensionSkill,
			ExtensionID: extensionID,
			Tool:        resolved.Tool,
		}, extensionspkg.BridgeRegisterOptions{
			AllowOriginalNameShadowBuiltin: true,
		})
		if err != nil {
			stableKey, stableErr := extensionspkg.StableToolKey(extensionspkg.ExtensionSkill, extensionID, original)
			if stableErr != nil {
				return err
			}
			if _, ok := registry.Get(stableKey); !ok {
				return err
			}
			binding = extensionspkg.BridgeBinding{
				Source:       extensionspkg.ExtensionSkill,
				ExtensionID:  extensionID,
				OriginalName: original,
				StableKey:    stableKey,
			}
		}
		desired[binding.StableKey] = struct{}{}
	}

	for toolKey := range r.activeSkillBridgeToolKeys {
		if _, keep := desired[toolKey]; keep {
			continue
		}
		if err := registry.Unregister(toolKey); err != nil {
			var regErr *toolspkg.RegistryError
			if !errors.As(err, &regErr) || regErr.Code != toolspkg.RegistryErrorNotFound {
				return err
			}
		}
		delete(r.activeSkillBridgeToolKeys, toolKey)
	}
	for toolKey := range desired {
		r.activeSkillBridgeToolKeys[toolKey] = struct{}{}
	}

	return nil
}

func (r *Runner) clearActiveSkillBridgesLocked(registry *toolspkg.Registry) {
	if registry != nil {
		for toolKey := range r.activeSkillBridgeToolKeys {
			_ = registry.Unregister(toolKey)
		}
	}
	r.activeSkillBridgeID = ""
	r.activeSkillBridgeToolKeys = map[string]struct{}{}
}

func normalizeBridgePolicyItem(item string) string {
	return strings.TrimSpace(item)
}

func shouldBridgePolicyItem(item string) bool {
	if item == "" {
		return false
	}
	if strings.Contains(item, ":") {
		return false
	}
	_, blocked := nonBridgedBuiltinTools[strings.ToLower(item)]
	return !blocked
}

package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bytemind/internal/agent"
	"bytemind/internal/assets"
	"bytemind/internal/llm"
	"bytemind/internal/session"
)

var imagePlaceholderPattern = regexp.MustCompile(`\[Image #(\d+)\]`)

type inputMutationClass string

const (
	inputMutationOrdinary    inputMutationClass = "ordinary"
	inputMutationPasteEmpty  inputMutationClass = "paste_empty"
	inputMutationPasteFilled inputMutationClass = "paste_filled"

	ctrlVKeyName    = "ctrl+v"
	ctrlVMarkerRune = "\x16"
)

type clipboardImageReader interface {
	ReadImage(ctx context.Context) (mediaType string, data []byte, fileName string, err error)
}

type defaultClipboardImageReader struct{}

func (defaultClipboardImageReader) ReadImage(ctx context.Context) (string, []byte, string, error) {
	if runtime.GOOS != "windows" {
		return "", nil, "", llm.WrapError("clipboard", llm.ErrorCodeClipboardUnavailable, fmt.Errorf("clipboard image is only supported on windows in this build"))
	}

	script := strings.Join([]string{
		"Add-Type -AssemblyName System.Windows.Forms",
		"Add-Type -AssemblyName System.Drawing",
		"if (-not [System.Windows.Forms.Clipboard]::ContainsImage()) { return '' }",
		"$img = [System.Windows.Forms.Clipboard]::GetImage()",
		"if ($null -eq $img) { return '' }",
		"$ms = New-Object System.IO.MemoryStream",
		"$img.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)",
		"[Convert]::ToBase64String($ms.ToArray())",
	}, "; ")

	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil {
		return "", nil, "", llm.WrapError("clipboard", llm.ErrorCodeClipboardUnavailable, fmt.Errorf("clipboard image read failed: %s", strings.TrimSpace(string(out))))
	}
	encoded := strings.TrimSpace(string(out))
	if encoded == "" {
		return "", nil, "", llm.WrapError("clipboard", llm.ErrorCodeClipboardUnavailable, fmt.Errorf("clipboard has no image"))
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", nil, "", llm.WrapError("clipboard", llm.ErrorCodeImageDecodeFailed, err)
	}
	if len(data) == 0 {
		return "", nil, "", llm.WrapError("clipboard", llm.ErrorCodeImageDecodeFailed, fmt.Errorf("clipboard image is empty"))
	}
	return "image/png", data, "clipboard.png", nil
}

func nextSessionImageID(sess *session.Session) int {
	if sess == nil {
		return 1
	}
	maxID := 0
	for _, meta := range sess.Conversation.Assets.Images {
		if meta.ImageID > maxID {
			maxID = meta.ImageID
		}
	}
	if maxID < 0 {
		return 1
	}
	return maxID + 1
}

func (m *model) ensureSessionImageAssets() {
	if m == nil || m.sess == nil {
		return
	}
	if m.sess.Conversation.Assets.Images == nil {
		m.sess.Conversation.Assets.Images = make(map[llm.AssetID]session.ImageAssetMeta, 8)
	}
}

func (m *model) applyInputImagePipeline(before, after, source string) (string, string) {
	class, prefix, inserted, suffix := classifyInputMutation(before, after, source)
	if class != inputMutationPasteFilled {
		return after, ""
	}

	paths := extractImagePathsFromChunk(inserted, m.workspace)
	if len(paths) == 0 {
		return after, ""
	}

	placeholders := make([]string, 0, len(paths))
	notes := make([]string, 0, len(paths))
	for _, path := range paths {
		placeholder, note, ok := m.ingestImageFromPath(path)
		if !ok {
			notes = append(notes, note)
			continue
		}
		placeholders = append(placeholders, placeholder)
	}
	if len(placeholders) == 0 {
		if len(notes) > 0 {
			return after, notes[0]
		}
		return after, ""
	}

	replacement := strings.Join(placeholders, " ")
	updated := after[:prefix] + replacement + after[len(after)-suffix:]
	m.syncInputImageRefs(updated)
	note := fmt.Sprintf("Attached %d image(s): %s", len(placeholders), strings.Join(placeholders, ", "))
	if len(notes) > 0 {
		note += "; " + notes[0]
	}
	return updated, note
}

func (m *model) handleEmptyClipboardPaste() string {
	if m == nil || m.clipboard == nil {
		return "Clipboard image is unavailable in current environment."
	}
	mediaType, data, fileName, err := m.clipboard.ReadImage(context.Background())
	if err != nil {
		return err.Error()
	}
	placeholder, note, ok := m.ingestImageBinary(mediaType, fileName, data)
	if !ok {
		return note
	}

	current := m.input.Value()
	updated := placeholder
	if strings.TrimSpace(current) != "" {
		updated = current + " " + placeholder
	}
	m.setInputValue(updated)
	m.syncInputImageRefs(updated)
	if note != "" {
		return note
	}
	return "Attached image from clipboard: " + placeholder
}

func (m *model) ingestImageFromPath(path string) (string, string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Sprintf("failed to read image %s: %v", path, err), false
	}
	mediaType, ok := mediaTypeFromPath(path)
	if !ok {
		return "", fmt.Sprintf("unsupported image format: %s", filepath.Ext(path)), false
	}
	return m.ingestImageBinary(mediaType, filepath.Base(path), data)
}

func (m *model) ingestImageBinary(mediaType, fileName string, data []byte) (string, string, bool) {
	if m == nil || m.sess == nil {
		return "", "image ingest failed: session unavailable", false
	}
	if m.imageStore == nil {
		return "", "image ingest failed: image store unavailable", false
	}
	if len(data) == 0 {
		return "", "image ingest failed: empty image payload", false
	}

	m.ensureSessionImageAssets()
	if m.nextImageID <= 0 {
		m.nextImageID = nextSessionImageID(m.sess)
	}
	imageID := m.nextImageID
	meta, err := m.imageStore.PutImage(context.Background(), assets.PutImageInput{
		SessionID: m.sess.ID,
		ImageID:   imageID,
		MediaType: mediaType,
		FileName:  fileName,
		Data:      data,
	})
	if err != nil {
		return "", err.Error(), false
	}

	assetID := meta.AssetID
	if strings.TrimSpace(string(assetID)) == "" {
		assetID = assets.AssetID(m.sess.ID, meta.ImageID)
	}
	m.sess.Conversation.Assets.Images[assetID] = session.ImageAssetMeta{
		ImageID:   meta.ImageID,
		MediaType: meta.MediaType,
		FileName:  meta.FileName,
		CachePath: meta.CachePath,
		ByteSize:  meta.ByteSize,
		Width:     meta.Width,
		Height:    meta.Height,
	}
	if m.store != nil {
		if err := m.store.Save(m.sess); err != nil {
			return "", err.Error(), false
		}
	}

	if m.inputImageRefs == nil {
		m.inputImageRefs = make(map[int]llm.AssetID, 8)
	}
	if m.orphanedImages == nil {
		m.orphanedImages = make(map[llm.AssetID]time.Time, 8)
	}
	m.inputImageRefs[meta.ImageID] = assetID
	delete(m.orphanedImages, assetID)
	m.nextImageID = meta.ImageID + 1

	return placeholderForImageID(meta.ImageID), "", true
}

func (m *model) syncInputImageRefs(text string) {
	if m == nil {
		return
	}
	if m.inputImageRefs == nil {
		m.inputImageRefs = make(map[int]llm.AssetID, 8)
	}
	if m.orphanedImages == nil {
		m.orphanedImages = make(map[llm.AssetID]time.Time, 8)
	}

	referencedIDs := extractImagePlaceholderIDs(text)
	referencedSet := make(map[int]struct{}, len(referencedIDs))
	for _, id := range referencedIDs {
		referencedSet[id] = struct{}{}
		assetID, _, ok := m.findAssetByImageID(id)
		if ok {
			m.inputImageRefs[id] = assetID
			delete(m.orphanedImages, assetID)
		}
	}

	for id, assetID := range m.inputImageRefs {
		if _, ok := referencedSet[id]; ok {
			continue
		}
		delete(m.inputImageRefs, id)
		m.orphanedImages[assetID] = time.Now().UTC()
	}
}

func (m *model) buildPromptInput(raw string) (agent.RunPromptInput, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return agent.RunPromptInput{}, "", fmt.Errorf("prompt is empty")
	}
	m.syncInputImageRefs(raw)

	matches := imagePlaceholderPattern.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return agent.RunPromptInput{
			UserMessage: llm.NewUserTextMessage(raw),
			DisplayText: raw,
		}, raw, nil
	}

	parts := make([]llm.Part, 0, len(matches)*2+1)
	assetPayloads := make(map[llm.AssetID]llm.ImageAsset, len(matches))
	appendTextPart := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		parts = append(parts, llm.Part{Type: llm.PartText, Text: &llm.TextPart{Value: text}})
	}

	last := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		idStart, idEnd := match[2], match[3]
		appendTextPart(raw[last:start])

		imageID, err := strconv.Atoi(raw[idStart:idEnd])
		if err != nil {
			appendTextPart(raw[start:end])
			last = end
			continue
		}
		assetID, _, ok := m.findAssetByImageID(imageID)
		if !ok {
			appendTextPart(raw[start:end])
			last = end
			continue
		}

		if m.imageStore == nil {
			appendTextPart(fmt.Sprintf("[Image #%d unavailable]", imageID))
			last = end
			continue
		}
		blob, err := m.imageStore.GetImageByAssetID(context.Background(), m.sess.ID, assetID)
		if err != nil {
			appendTextPart(fmt.Sprintf("[Image #%d unavailable]", imageID))
			last = end
			continue
		}
		assetPayloads[assetID] = llm.ImageAsset{MediaType: blob.MediaType, Data: blob.Data}
		parts = append(parts, llm.Part{Type: llm.PartImageRef, Image: &llm.ImagePartRef{AssetID: assetID}})
		last = end
	}
	appendTextPart(raw[last:])

	if len(parts) == 0 {
		parts = append(parts, llm.Part{Type: llm.PartText, Text: &llm.TextPart{Value: raw}})
	}

	userMessage := llm.Message{Role: llm.RoleUser, Parts: parts}
	userMessage.Normalize()
	if err := llm.ValidateMessage(userMessage); err != nil {
		return agent.RunPromptInput{}, "", err
	}

	return agent.RunPromptInput{
		UserMessage: userMessage,
		Assets:      assetPayloads,
		DisplayText: raw,
	}, raw, nil
}

func (m *model) findAssetByImageID(imageID int) (llm.AssetID, session.ImageAssetMeta, bool) {
	if m == nil || m.sess == nil {
		return "", session.ImageAssetMeta{}, false
	}
	for assetID, meta := range m.sess.Conversation.Assets.Images {
		if meta.ImageID == imageID {
			return assetID, meta, true
		}
	}
	return "", session.ImageAssetMeta{}, false
}

func classifyInputMutation(before, after, source string) (inputMutationClass, int, string, int) {
	prefix, inserted, suffix := insertionDiff(before, after)
	cleanInserted := strings.ReplaceAll(inserted, ctrlVMarkerRune, "")
	pasteSignal := isCtrlVKey(source) || strings.Contains(strings.ToLower(source), "paste") || strings.Contains(cleanInserted, "\n") || len(cleanInserted) > 1
	if shouldTriggerClipboardImagePaste(before, after, source) {
		return inputMutationPasteEmpty, prefix, inserted, suffix
	}
	if pasteSignal && strings.TrimSpace(cleanInserted) != "" {
		return inputMutationPasteFilled, prefix, inserted, suffix
	}
	return inputMutationOrdinary, prefix, inserted, suffix
}

func isCtrlVKey(source string) bool {
	source = strings.TrimSpace(source)
	return strings.EqualFold(source, ctrlVKeyName) ||
		source == ctrlVMarkerRune ||
		source == "["+ctrlVMarkerRune+"]"
}

func shouldTriggerClipboardImagePaste(before, after, source string) bool {
	if !isCtrlVKey(source) {
		return false
	}
	_, inserted, _ := insertionDiff(before, after)
	cleanInserted := strings.ReplaceAll(inserted, ctrlVMarkerRune, "")
	return strings.TrimSpace(cleanInserted) == ""
}

func stripCtrlVMarker(text string) (string, bool) {
	cleaned := strings.ReplaceAll(text, ctrlVMarkerRune, "")
	return cleaned, cleaned != text
}

func insertionDiff(before, after string) (prefix int, inserted string, suffix int) {
	prefix = lenCommonPrefix(before, after)
	beforeTail := before[prefix:]
	afterTail := after[prefix:]
	suffix = lenCommonSuffix(beforeTail, afterTail)
	if suffix > len(afterTail) {
		suffix = len(afterTail)
	}
	if suffix > len(beforeTail) {
		suffix = len(beforeTail)
	}
	inserted = afterTail[:len(afterTail)-suffix]
	return prefix, inserted, suffix
}

func lenCommonSuffix(a, b string) int {
	limit := min(len(a), len(b))
	for i := 0; i < limit; i++ {
		if a[len(a)-1-i] != b[len(b)-1-i] {
			return i
		}
	}
	return limit
}

func extractImagePathsFromChunk(chunk, workspace string) []string {
	tokens := splitPathTokens(chunk)
	if len(tokens) == 0 {
		return nil
	}

	paths := make([]string, 0, len(tokens))
	candidateCount := 0
	for _, token := range tokens {
		token = strings.TrimSpace(strings.Trim(token, `"'`))
		if token == "" {
			continue
		}
		candidateCount++

		resolved := token
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(workspace, token)
		}
		resolved = filepath.Clean(resolved)
		info, err := os.Stat(resolved)
		if err != nil || info.IsDir() {
			continue
		}
		if _, ok := mediaTypeFromPath(resolved); !ok {
			continue
		}
		paths = append(paths, resolved)
	}

	if candidateCount == 0 || len(paths) != candidateCount {
		return nil
	}
	return paths
}

func splitPathTokens(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	result := make([]string, 0, 8)
	var b strings.Builder
	quote := rune(0)
	for _, r := range raw {
		switch {
		case quote == 0 && (r == '\'' || r == '"'):
			quote = r
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '\n' || r == '\r' || r == '\t' || r == ' '):
			if b.Len() > 0 {
				result = append(result, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		result = append(result, b.String())
	}
	return result
}

func mediaTypeFromPath(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".webp":
		return "image/webp", true
	case ".gif":
		return "image/gif", true
	default:
		return "", false
	}
}

func extractImagePlaceholderIDs(text string) []int {
	matches := imagePlaceholderPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	ids := make([]int, 0, len(matches))
	seen := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id, err := strconv.Atoi(match[1])
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func placeholderForImageID(id int) string {
	return fmt.Sprintf("[Image #%d]", id)
}

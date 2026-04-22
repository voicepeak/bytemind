package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// SimpleMarkdownRenderer - 简约的Markdown渲染器
type SimpleMarkdownRenderer struct {
	style *MarkdownStyleConfig
	width int
	cache *SimpleCache
}

// MarkdownStyleConfig - 简约的样式配置
type MarkdownStyleConfig struct {
	// 基础颜色
	TextColor    lipgloss.Color
	AccentColor  lipgloss.Color
	MutedColor   lipgloss.Color
	SuccessColor lipgloss.Color
	WarningColor lipgloss.Color
	ErrorColor   lipgloss.Color

	// 代码块颜色
	CodeBgColor     lipgloss.Color
	CodeBorderColor lipgloss.Color

	// 高亮颜色
	HighlightColors map[string]lipgloss.Color
}

// SimpleCache - 简约的缓存机制
type SimpleCache struct {
	cache map[string]string
	mutex sync.RWMutex
}

// NewSimpleCache 创建新的简单缓存
func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		cache: make(map[string]string),
	}
}

// Get 从缓存获取值
func (c *SimpleCache) Get(key string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	value, ok := c.cache[key]
	return value, ok
}

// Set 设置缓存值
func (c *SimpleCache) Set(key, value string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[key] = value
}

// NewSimpleMarkdownRenderer 创建新的简约Markdown渲染器
func NewSimpleMarkdownRenderer(width int) *SimpleMarkdownRenderer {
	// 默认样式配置
	config := &MarkdownStyleConfig{
		TextColor:       semanticColors.TextBase,
		AccentColor:     semanticColors.Accent,
		MutedColor:      semanticColors.TextMuted,
		SuccessColor:    semanticColors.Success,
		WarningColor:    semanticColors.Warning,
		ErrorColor:      semanticColors.Danger,
		CodeBgColor:     semanticColors.CodeBg,
		CodeBorderColor: semanticColors.CodeBorder,
		HighlightColors: map[string]lipgloss.Color{
			"yellow": semanticColors.HighlightYellow,
			"blue":   semanticColors.HighlightBlue,
			"green":  semanticColors.HighlightGreen,
			"red":    semanticColors.HighlightRed,
			"purple": semanticColors.HighlightPurple,
			"orange": semanticColors.HighlightOrange,
		},
	}

	return &SimpleMarkdownRenderer{
		style: config,
		width: width,
		cache: NewSimpleCache(),
	}
}

// Render 渲染Markdown文本
func (r *SimpleMarkdownRenderer) Render(markdown string) string {
	// 检查缓存
	if cached, ok := r.cache.Get(markdown); ok {
		return cached
	}

	// 解析Markdown
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	reader := text.NewReader([]byte(markdown))
	document := md.Parser().Parse(reader)

	// 渲染
	result := r.renderNode(document, reader.Source())

	// 缓存结果
	r.cache.Set(markdown, result)

	return result
}

// renderNode 渲染单个节点
func (r *SimpleMarkdownRenderer) renderNode(node gast.Node, source []byte) string {
	var result strings.Builder

	// 遍历子节点
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		result.WriteString(r.renderNodeInternal(child, source))
	}

	return result.String()
}

// renderNodeInternal 渲染节点内部实现
func (r *SimpleMarkdownRenderer) renderNodeInternal(node gast.Node, source []byte) string {
	switch n := node.(type) {
	case *gast.Heading:
		return r.renderHeading(n, source)
	case *gast.Paragraph:
		return r.renderParagraph(n, source)
	case *gast.FencedCodeBlock:
		return r.renderCodeBlock(n, source)
	case *gast.CodeBlock:
		return r.renderCodeBlock(n, source)
	case *gast.List:
		return r.renderList(n, source)
	case *gast.Blockquote:
		return r.renderBlockquote(n, source)
	// Skip table rendering for simplicity
	case *gast.ThematicBreak:
		return r.renderThematicBreak()
	case *gast.Text:
		return r.processText(string(n.Text(source)))
	case *gast.Emphasis:
		return r.renderEmphasis(n, source)
	case *gast.String:
		return r.processText(string(n.Value))
	case *gast.Link:
		return r.renderLink(n, source)
	case *gast.CodeSpan:
		return r.renderCodeSpan(n, source)
	case *gast.HTMLBlock:
		return "" // 跳过HTML块
	default:
		return r.renderNode(node, source)
	}
}

// renderHeading 渲染标题
func (r *SimpleMarkdownRenderer) renderHeading(node *gast.Heading, source []byte) string {
	text := r.collectText(node, source)
	style := r.style.Heading(node.Level)
	return fmt.Sprintf("%s\n", style.Render(text))
}

// renderParagraph 渲染段落
func (r *SimpleMarkdownRenderer) renderParagraph(node *gast.Paragraph, source []byte) string {
	text := r.collectText(node, source)
	processedText := r.processHighlights(text)
	return fmt.Sprintf("%s\n\n", r.style.Text().Render(processedText))
}

// renderCodeBlock 渲染代码块
func (r *SimpleMarkdownRenderer) renderCodeBlock(node gast.Node, source []byte) string {
	var language string
	var code string

	switch n := node.(type) {
	case *gast.FencedCodeBlock:
		language = string(n.Language(source))
		code = string(n.Lines().Value(source))
	default:
		code = string(node.Lines().Value(source))
	}

	// 添加语法高亮
	highlightedCode := r.highlightCode(code, language)

	// 构建代码块
	border := strings.Repeat("─", r.width-4)
	result := fmt.Sprintf("┌─ %s ─┐\n", language)

	lines := strings.Split(highlightedCode, "\n")
	for _, line := range lines {
		result += fmt.Sprintf("│ %-*s │\n", r.width-6, line)
	}

	result += fmt.Sprintf("└─%s─┘\n", border)

	return r.style.Code().Render(result) + "\n"
}

// highlightCode 代码语法高亮
func (r *SimpleMarkdownRenderer) highlightCode(code, language string) string {
	// 简化实现，不进行实际的语法高亮
	return code
}

// renderList 渲染列表
func (r *SimpleMarkdownRenderer) renderList(node *gast.List, source []byte) string {
	var result strings.Builder

	isOrdered := node.IsOrdered()
	start := node.Start
	index := start

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if item, ok := child.(*gast.ListItem); ok {
			text := r.collectText(item, source)

			// 检查是否是任务列表项
			if firstChild := item.FirstChild(); firstChild != nil {
				if textNode, ok := firstChild.(*gast.Text); ok {
					textStr := string(textNode.Text(source))
					if strings.HasPrefix(textStr, "[ ] ") {
						text = "☐ " + textStr[3:] + "\n"
					} else if strings.HasPrefix(textStr, "[x] ") {
						text = "☑ " + textStr[3:] + "\n"
					}
				}
			}

			if isOrdered {
				result.WriteString(fmt.Sprintf("%d. %s\n", index, text))
				index++
			} else {
				result.WriteString(fmt.Sprintf("• %s\n", text))
			}
		}
	}

	return result.String() + "\n"
}

// renderBlockquote 渲染引用块
func (r *SimpleMarkdownRenderer) renderBlockquote(node *gast.Blockquote, source []byte) string {
	text := r.collectText(node, source)
	lines := strings.Split(text, "\n")

	var result strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result.WriteString(fmt.Sprintf("▍ %s\n", line))
		} else {
			result.WriteString("▍\n")
		}
	}

	return result.String() + "\n"
}

// renderTable 渲染表格
func (r *SimpleMarkdownRenderer) renderTable(node gast.Node, source []byte) string {
	// 简化的表格渲染
	return r.collectText(node, source) + "\n"
}

// renderThematicBreak 渲染分隔线
func (r *SimpleMarkdownRenderer) renderThematicBreak() string {
	rule := strings.Repeat("─", r.width)
	return fmt.Sprintf("%s\n\n", rule)
}

// renderEmphasis 渲染强调
func (r *SimpleMarkdownRenderer) renderEmphasis(node *gast.Emphasis, source []byte) string {
	text := r.collectText(node, source)
	return r.style.Emphasis().Render(text)
}

// renderStrong 渲染粗体
func (r *SimpleMarkdownRenderer) renderStrong(node *gast.Emphasis, source []byte) string {
	text := r.collectText(node, source)
	return r.style.Strong().Render(text)
}

// renderDel 渲染删除线
func (r *SimpleMarkdownRenderer) renderDel(node *gast.Text, source []byte) string {
	text := string(node.Text(source))
	return r.style.Strikethrough().Render(text)
}

// renderLink 渲染链接
func (r *SimpleMarkdownRenderer) renderLink(node *gast.Link, source []byte) string {
	text := r.collectText(node, source)
	url := string(node.Destination)
	return r.style.Link().Render(fmt.Sprintf("%s (%s)", text, url))
}

// renderCodeSpan 渲染内联代码
func (r *SimpleMarkdownRenderer) renderCodeSpan(node *gast.CodeSpan, source []byte) string {
	text := string(node.Text(source))
	return r.style.InlineCode().Render(text)
}

// processText 处理文本
func (r *SimpleMarkdownRenderer) processText(text string) string {
	return text // 简化实现
}

// processHighlights 处理文本高亮
func (r *SimpleMarkdownRenderer) processHighlights(text string) string {
	// 处理 ==text== 高亮
	text = strings.ReplaceAll(text, "==", "")

	// 简化实现，只处理基本的高亮
	return text
}

// collectText 收集节点中的所有文本
func (r *SimpleMarkdownRenderer) collectText(node gast.Node, source []byte) string {
	var result strings.Builder

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if textNode, ok := child.(*gast.Text); ok {
			result.WriteString(string(textNode.Text(source)))
		} else {
			result.WriteString(r.renderNodeInternal(child, source))
		}
	}

	return result.String()
}

// 样式工厂方法

// Heading 返回标题样式
func (s *MarkdownStyleConfig) Heading(level int) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(s.TextColor).Bold(true)

	switch level {
	case 1:
		return style.Foreground(s.AccentColor).Underline(true)
	case 2:
		return style.Foreground(s.AccentColor)
	case 3:
		return style.Foreground(s.MutedColor)
	default:
		return style.Foreground(s.MutedColor)
	}
}

// Text 返回普通文本样式
func (s *MarkdownStyleConfig) Text() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.TextColor)
}

// Code 返回代码块样式
func (s *MarkdownStyleConfig) Code() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(s.TextColor).
		Background(s.CodeBgColor).
		Padding(0, 1)
}

// Emphasis 返回强调样式
func (s *MarkdownStyleConfig) Emphasis() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.AccentColor).Italic(true)
}

// Strong 返回粗体样式
func (s *MarkdownStyleConfig) Strong() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.TextColor).Bold(true)
}

// Strikethrough 返回删除线样式
func (s *MarkdownStyleConfig) Strikethrough() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.MutedColor).Strikethrough(true)
}

// Link 返回链接样式
func (s *MarkdownStyleConfig) Link() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.AccentColor).Underline(true)
}

// InlineCode 返回内联代码样式
func (s *MarkdownStyleConfig) InlineCode() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(s.TextColor).
		Background(s.CodeBgColor).
		Bold(true)
}

// Highlight 返回高亮样式
func (s *MarkdownStyleConfig) Highlight(color string) lipgloss.Style {
	if c, ok := s.HighlightColors[color]; ok {
		return lipgloss.NewStyle().Background(c)
	}
	return lipgloss.NewStyle().Background(s.HighlightColors["yellow"])
}

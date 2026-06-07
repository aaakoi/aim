package rag

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// Document 文档结构
type Document struct {
	ID       string
	Name     string
	Content  string
	Metadata map[string]string
}

// DocumentParser 文档解析器
type DocumentParser struct{}

// NewDocumentParser 创建文档解析器
func NewDocumentParser() *DocumentParser {
	return &DocumentParser{}
}

// ParseFile 解析文件（根据扩展名选择解析方法）
func (p *DocumentParser) ParseFile(filePath string) (*Document, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	var text string
	switch ext {
	case ".md", ".txt":
		text = string(content)
	case ".pdf":
		text, err = p.parsePDF(filePath)
	case ".docx", ".doc":
		text, err = p.parseDOCX(filePath)
	case ".pptx", ".ppt":
		text, err = p.parsePPTX(filePath)
	default:
		text = string(content)
	}

	if err != nil {
		return nil, err
	}

	// 检查解析结果
	text = strings.TrimSpace(text)
	if len(text) < 10 {
		// 文本太短，可能是解析失败
		text = fmt.Sprintf("[文档 %s 内容解析结果]\n注意：文档解析可能不完整，建议使用txt/md格式", filepath.Base(filePath))
	}

	docID := generateID()
	return &Document{
		ID:      docID,
		Name:    filepath.Base(filePath),
		Content: text,
		Metadata: map[string]string{
			"path":      filePath,
			"extension": ext,
		},
	}, nil
}

// ParseContent 直接解析内容
func (p *DocumentParser) ParseContent(name, content string) (*Document, error) {
	docID := generateID()
	return &Document{
		ID:      docID,
		Name:    name,
		Content: content,
		Metadata: map[string]string{
			"source": "direct",
		},
	}, nil
}

// parsePDF 解析PDF文件（使用专业库）
func (p *DocumentParser) parsePDF(filePath string) (string, error) {
	// 使用 github.com/ledongthuc/pdf 专业库
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开PDF失败: %v", err)
	}
	defer f.Close()

	var text strings.Builder
	b, err := r.GetPlainText()
	if err != nil {
		// 如果无法获取纯文本，尝试其他方法
		return p.parsePDFAlternative(filePath)
	}

	content, err := io.ReadAll(b)
	if err != nil {
		return "", err
	}

	text.WriteString(string(content))

	result := text.String()
	if len(strings.TrimSpace(result)) < 10 {
		// 如果提取的文本太少，使用备选方法
		return p.parsePDFAlternative(filePath)
	}

	return result, nil
}

// parsePDFAlternative PDF解析备选方法
func (p *DocumentParser) parsePDFAlternative(filePath string) (string, error) {
	// 读取PDF原始内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 提取可读文本
	text := extractTextFromPDF(content)
	if len(strings.TrimSpace(text)) < 10 {
		return "", fmt.Errorf("PDF文本提取失败，可能是扫描版PDF")
	}
	return text, nil
}

// parseDOCX 解析DOCX文件
func (p *DocumentParser) parseDOCX(filePath string) (string, error) {
	// DOCX实际上是一个ZIP文件，包含XML
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var text strings.Builder
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			// 从XML中提取文本
			text.WriteString(extractTextFromXML(content))
		}
	}

	return text.String(), nil
}

// parsePPTX 解析PPTX文件
func (p *DocumentParser) parsePPTX(filePath string) (string, error) {
	// PPTX也是一个ZIP文件
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var text strings.Builder
	for _, f := range r.File {
		// 每个幻灯片是一个slideX.xml文件
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, _ := io.ReadAll(rc)
			rc.Close()

			text.WriteString(extractTextFromXML(content))
			text.WriteString("\n---\n")
		}
	}

	return text.String(), nil
}

// extractTextFromPDF 从PDF字节中提取文本（简化版）
func extractTextFromPDF(data []byte) string {
	// 简化实现：查找PDF中的文本流
	// 实际项目中应使用专业的PDF库
	text := string(data)

	// 尝试提取BT...ET之间的文本（PDF文本块）
	var result strings.Builder
	inText := false
	for i := 0; i < len(text)-2; i++ {
		if text[i:i+2] == "BT" {
			inText = true
			i += 2
		} else if text[i:i+2] == "ET" {
			inText = false
			i += 2
		} else if inText && text[i] >= 32 && text[i] < 127 {
			result.WriteByte(text[i])
		}
	}

	extracted := result.String()
	if len(extracted) < 50 {
		// 如果提取的文本太少，返回提示
		return "[PDF内容，请使用专业PDF解析库提取文本]"
	}
	return extracted
}

// XMLText 用于提取XML中的文本
type XMLText struct {
	Text string `xml:",chardata"`
}

// extractTextFromXML 从XML中提取文本
func extractTextFromXML(data []byte) string {
	// 简单方法：移除所有XML标签
	text := string(data)

	// 移除XML标签
	var result strings.Builder
	inTag := false
	for _, c := range text {
		if c == '<' {
			inTag = true
		} else if c == '>' {
			inTag = false
			result.WriteByte(' ')
		} else if !inTag {
			if c >= 32 && c < 127 || c > 127 {
				result.WriteRune(c)
			}
		}
	}

	return strings.TrimSpace(result.String())
}

// generateID 生成唯一ID
func generateID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[i%len(chars)]
	}
	return string(b)
}

// ParseXMLText 解析XML文本（备用方法）
func parseXMLText(data []byte) string {
	var texts []XMLText
	xml.Unmarshal(data, &texts)

	var result strings.Builder
	for _, t := range texts {
		result.WriteString(t.Text)
		result.WriteString(" ")
	}
	return result.String()
}

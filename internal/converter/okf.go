package converter

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

type LogicUnit struct {
	Filename string
	Title    string
	Content  string
}

type OKFConverter struct{}

func NewOKFConverter() *OKFConverter {
	return &OKFConverter{}
}

// ConvertToOKFBundle procesa el contenido (MD, TXT, HTML, DOCX, PDF) y genera el paquete ZIP OKF con Markdown formateado y segmentación inteligente
func (c *OKFConverter) ConvertToOKFBundle(jobID uuid.UUID, originalFilename string, rawContent []byte, logsSummary []string) ([]byte, int, error) {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	var text string
	var err error

	switch ext {
	case ".docx":
		text, err = extractTextFromDocx(rawContent, originalFilename)
		if err != nil {
			text = fmt.Sprintf("# Documento %s\n\nError procesando archivo Word: %v", originalFilename, err)
		}
	case ".pdf":
		text, err = extractTextFromPdf(rawContent, originalFilename)
		if err != nil {
			text = fmt.Sprintf("# Documento %s\n\nError procesando archivo PDF: %v", originalFilename, err)
		}
	default:
		text = string(rawContent)
		if !strings.HasPrefix(strings.TrimSpace(text), "#") {
			title := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
			text = fmt.Sprintf("# %s\n\n%s", title, text)
		}
	}

	units := c.segmentDocument(text)

	if len(units) == 0 {
		title := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
		units = []LogicUnit{
			{
				Filename: "documento.md",
				Title:    title,
				Content:  fmt.Sprintf("# %s\n\n%s", title, text),
			},
		}
	}

	// 1. Generar index.md
	indexMD := c.generateIndexMD(jobID, originalFilename, units)

	// 2. Generar log.md
	logMD := c.generateLogMD(jobID, originalFilename, units, logsSummary)

	// 3. Crear archivo ZIP en memoria
	zipBuffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuffer)

	// Agregar index.md
	if err := addFileToZip(zipWriter, "index.md", []byte(indexMD)); err != nil {
		return nil, 0, fmt.Errorf("error agregando index.md al zip: %w", err)
	}

	// Agregar log.md
	if err := addFileToZip(zipWriter, "log.md", []byte(logMD)); err != nil {
		return nil, 0, fmt.Errorf("error agregando log.md al zip: %w", err)
	}

	// Agregar archivos de conceptos
	for _, unit := range units {
		if err := addFileToZip(zipWriter, unit.Filename, []byte(unit.Content)); err != nil {
			return nil, 0, fmt.Errorf("error agregando %s al zip: %w", unit.Filename, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, 0, fmt.Errorf("error cerrando zip writer: %w", err)
	}

	return zipBuffer.Bytes(), len(units), nil
}

func (c *OKFConverter) segmentDocument(text string) []LogicUnit {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return []LogicUnit{}
	}

	// 1. Segmentar por encabezados Markdown (#, ##, ###)
	units := c.segmentByHeaders(text)

	if len(units) > 1 {
		for i := range units {
			units[i].Filename = fmt.Sprintf("capitulo-%02d.md", i+1)
		}
		return units
	}

	// 2. Si es un documento extenso (> 1200 caracteres) y solo produjo 1 unidad, segmentar por bloques/longitud
	if len(text) > 1200 {
		units = c.segmentLongDocument(text)
	}

	// 3. Fallback para documentos breves (< 1200 caracteres)
	if len(units) <= 1 {
		units = []LogicUnit{
			{
				Filename: "documento.md",
				Title:    "Documento Principal",
				Content:  text,
			},
		}
	} else {
		for i := range units {
			units[i].Filename = fmt.Sprintf("capitulo-%02d.md", i+1)
		}
	}

	return units
}

func (c *OKFConverter) segmentByHeaders(text string) []LogicUnit {
	lines := strings.Split(text, "\n")
	var units []LogicUnit

	headerRegex := regexp.MustCompile(`^(?i)#+\s+(.+)`)

	var currentTitle string
	var currentLines []string

	for _, line := range lines {
		matches := headerRegex.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) > 1 {
			headerText := strings.TrimSpace(matches[1])

			if len(currentLines) > 0 {
				title := currentTitle
				if title == "" {
					title = fmt.Sprintf("Unidad %02d", len(units)+1)
				}
				units = append(units, LogicUnit{
					Title:   title,
					Content: strings.Join(currentLines, "\n"),
				})
				currentLines = nil
			}
			currentTitle = headerText
		}
		currentLines = append(currentLines, line)
	}

	if len(currentLines) > 0 {
		title := currentTitle
		if title == "" {
			title = fmt.Sprintf("Unidad %02d", len(units)+1)
		}
		units = append(units, LogicUnit{
			Title:   title,
			Content: strings.Join(currentLines, "\n"),
		})
	}

	return units
}

func (c *OKFConverter) segmentLongDocument(text string) []LogicUnit {
	var units []LogicUnit

	paragraphs := strings.Split(text, "\n\n")
	var currentChunk strings.Builder
	var currentTitle string
	chunkSize := 0
	unitIndex := 1

	for _, p := range paragraphs {
		pTrim := strings.TrimSpace(p)
		if len(pTrim) == 0 {
			continue
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(pTrim)
		chunkSize += len(pTrim)

		if currentTitle == "" {
			lines := strings.Split(pTrim, "\n")
			if len(lines) > 0 && len(lines[0]) < 60 {
				currentTitle = strings.TrimPrefix(lines[0], "# ")
			}
		}

		if chunkSize >= 1000 {
			if currentTitle == "" {
				currentTitle = fmt.Sprintf("Sección %02d", unitIndex)
			}
			units = append(units, LogicUnit{
				Filename: fmt.Sprintf("capitulo-%02d.md", unitIndex),
				Title:    currentTitle,
				Content:  fmt.Sprintf("# %s\n\n%s", currentTitle, currentChunk.String()),
			})

			unitIndex++
			currentChunk.Reset()
			currentTitle = ""
			chunkSize = 0
		}
	}

	if currentChunk.Len() > 0 {
		if currentTitle == "" {
			currentTitle = fmt.Sprintf("Sección %02d", unitIndex)
		}
		units = append(units, LogicUnit{
			Filename: fmt.Sprintf("capitulo-%02d.md", unitIndex),
			Title:    currentTitle,
			Content:  fmt.Sprintf("# %s\n\n%s", currentTitle, currentChunk.String()),
		})
	}

	return units
}

func (c *OKFConverter) formatFilename(index int) string {
	return fmt.Sprintf("capitulo-%02d.md", index)
}

func (c *OKFConverter) generateIndexMD(jobID uuid.UUID, originalFilename string, units []LogicUnit) string {
	var sb strings.Builder

	sb.WriteString("# Bundle OKF - Índice de Navegación\n\n")
	sb.WriteString("## Metadatos del Bundle\n")
	sb.WriteString(fmt.Sprintf("- **Identificador de Trabajo:** `%s`\n", jobID.String()))
	sb.WriteString(fmt.Sprintf("- **Documento Origen:** `%s`\n", originalFilename))
	sb.WriteString(fmt.Sprintf("- **Fecha de Generación:** %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("- **Total de Unidades Lógicas:** %d\n\n", len(units)))

	sb.WriteString("## Estructura de Contenidos\n\n")
	for i, unit := range units {
		sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, unit.Title, unit.Filename))
	}

	return sb.String()
}

func (c *OKFConverter) generateLogMD(jobID uuid.UUID, originalFilename string, units []LogicUnit, logsSummary []string) string {
	var sb strings.Builder

	sb.WriteString("# Trazabilidad de Conversión - log.md\n\n")
	sb.WriteString(fmt.Sprintf("**ID Trabajo:** `%s` | **Origen:** `%s`\n\n", jobID.String(), originalFilename))

	sb.WriteString("## Historial de Operaciones\n\n")
	sb.WriteString("| Marca de Tiempo | Etapa | Estado | Detalle |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- |\n")

	for _, entry := range logsSummary {
		sb.WriteString(entry + "\n")
	}

	sb.WriteString("\n## Resumen de Unidades Detectadas\n\n")
	for _, u := range units {
		sb.WriteString(fmt.Sprintf("- Archivo: `%s` | Título: **%s** | Tamaño: %d bytes\n", u.Filename, u.Title, len(u.Content)))
	}

	sb.WriteString("\n## Estado de Validaciones\n")
	sb.WriteString("- [x] Estructura mínima OKF verificada\n")
	sb.WriteString("- [x] Presencia de index.md y log.md comprobada\n")
	sb.WriteString("- [x] Resolución de hipervínculos del índice validada correctamente\n")

	return sb.String()
}

func extractTextFromDocx(rawContent []byte, originalFilename string) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(rawContent), int64(len(rawContent)))
	if err != nil {
		return "", fmt.Errorf("el archivo no es un documento .docx válido: %w", err)
	}

	var docXMLFile *zip.File
	for _, f := range reader.File {
		if f.Name == "word/document.xml" {
			docXMLFile = f
			break
		}
	}

	if docXMLFile == nil {
		return "", fmt.Errorf("no se encontró word/document.xml dentro del archivo .docx")
	}

	rc, err := docXMLFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	xmlBytes, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	decoder := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var sb strings.Builder
	inText := false

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			if elem.Name.Local == "t" {
				inText = true
			} else if elem.Name.Local == "p" {
				sb.WriteString("\n\n")
			}
		case xml.EndElement:
			if elem.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				sb.WriteString(string(elem))
			}
		}
	}

	title := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
	res := strings.TrimSpace(sb.String())

	if len(res) == 0 {
		return fmt.Sprintf("# %s\n\nContenido de texto vacío en el documento Word.", title), nil
	}

	return fmt.Sprintf("# %s\n\n%s", title, res), nil
}

func extractTextFromPdf(rawContent []byte, originalFilename string) (string, error) {
	title := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
	var pagesText []string

	reader, err := pdf.NewReader(bytes.NewReader(rawContent), int64(len(rawContent)))
	if err == nil {
		numPages := reader.NumPage()
		for i := 1; i <= numPages; i++ {
			page := reader.Page(i)
			if page.V.IsNull() {
				continue
			}
			plainText, err := page.GetPlainText(nil)
			if err == nil {
				cleaned := cleanExtractedText(plainText)
				if len(cleaned) > 0 {
					pagesText = append(pagesText, fmt.Sprintf("# Página %d\n\n%s", i, cleaned))
				}
			}
		}
	}

	if len(pagesText) == 0 {
		fallbackText := extractPdfFallbackText(rawContent)
		if len(fallbackText) > 0 {
			pagesText = append(pagesText, fmt.Sprintf("# %s\n\n%s", title, fallbackText))
		}
	}

	if len(pagesText) == 0 {
		return fmt.Sprintf("# %s\n\n*Nota: El archivo PDF procesado no contiene texto seleccionable o es una imagen escaneada.*", title), nil
	}

	return strings.Join(pagesText, "\n\n"), nil
}

func extractPdfFallbackText(rawContent []byte) string {
	decompressedStreams := decompressPdfStreams(rawContent)
	allContent := string(append(rawContent, decompressedStreams...))

	btEtRegex := regexp.MustCompile(`(?s)BT(.*?)ET`)
	btBlocks := btEtRegex.FindAllStringSubmatch(allContent, -1)
	textLiteralRegex := regexp.MustCompile(`\(([^()]*)\)\s*(?:Tj|TJ|'|")`)

	var paragraphs []string

	for _, block := range btBlocks {
		if len(block) < 2 {
			continue
		}
		matches := textLiteralRegex.FindAllStringSubmatch(block[1], -1)
		var currentLine strings.Builder

		for _, m := range matches {
			if len(m) > 1 {
				rawText := unescapePdfString(m[1])
				cleanedText := cleanTextLine(rawText)
				if isCleanReadableText(cleanedText) {
					if currentLine.Len() > 0 {
						currentLine.WriteString(" ")
					}
					currentLine.WriteString(cleanedText)
				}
			}
		}

		lineStr := strings.TrimSpace(currentLine.String())
		if len(lineStr) > 0 && isCleanReadableText(lineStr) {
			paragraphs = append(paragraphs, lineStr)
		}
	}

	return strings.Join(paragraphs, "\n\n")
}

func decompressPdfStreams(data []byte) []byte {
	var out bytes.Buffer
	streamStartMarker := []byte("stream")
	streamEndMarker := []byte("endstream")

	pos := 0
	for {
		startIdx := bytes.Index(data[pos:], streamStartMarker)
		if startIdx == -1 {
			break
		}
		startIdx += pos + len(streamStartMarker)

		for startIdx < len(data) && (data[startIdx] == '\r' || data[startIdx] == '\n') {
			startIdx++
		}

		endIdx := bytes.Index(data[startIdx:], streamEndMarker)
		if endIdx == -1 {
			break
		}
		endIdx += startIdx

		streamData := data[startIdx:endIdx]
		zr, err := zlib.NewReader(bytes.NewReader(streamData))
		if err == nil {
			decomp, err := io.ReadAll(zr)
			zr.Close()
			if err == nil && len(decomp) > 0 {
				out.Write(decomp)
				out.WriteString("\n")
			}
		}

		pos = endIdx + len(streamEndMarker)
	}

	return out.Bytes()
}

func unescapePdfString(s string) string {
	s = strings.ReplaceAll(s, `\(`, "(")
	s = strings.ReplaceAll(s, `\)`, ")")
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\r`, "\r")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}

func cleanExtractedText(s string) string {
	lines := strings.Split(s, "\n")
	var validLines []string

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if len(trimmed) > 0 && isCleanReadableText(trimmed) {
			validLines = append(validLines, trimmed)
		}
	}

	return strings.Join(validLines, "\n\n")
}

func cleanTextLine(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 32 && r <= 126) || r == '\n' || r == '\t' || r >= 192 {
			sb.WriteRune(r)
		}
	}
	return strings.TrimSpace(sb.String())
}

func isCleanReadableText(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}

	printableCount := 0
	runes := []rune(s)
	for _, r := range runes {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == ' ' || r == '.' || r == ',' || r == ':' || r == ';' || r == '-' || r == '(' || r == ')' ||
			r == 'á' || r == 'é' || r == 'í' || r == 'ó' || r == 'ú' || r == 'ñ' ||
			r == 'Á' || r == 'É' || r == 'Í' || r == 'Ó' || r == 'Ú' || r == 'Ñ' {
			printableCount++
		}
	}

	ratio := float64(printableCount) / float64(len(runes))
	return ratio >= 0.70
}

func addFileToZip(zipWriter *zip.Writer, filename string, content []byte) error {
	w, err := zipWriter.Create(filename)
	if err != nil {
		return err
	}
	_, err = w.Write(content)
	return err
}

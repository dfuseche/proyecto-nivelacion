package converter

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestOKFConverter_SingleUnit(t *testing.T) {
	conv := NewOKFConverter()
	jobID := uuid.New()
	content := []byte("Este es un documento simple sin encabezados ni divisiones.")

	zipBytes, unitsCount, err := conv.ConvertToOKFBundle(jobID, "simple.txt", content, []string{"| 12:00:00 | RECEPCION | INFO | Test |"})
	if err != nil {
		t.Fatalf("error convirtiendo documento breve: %v", err)
	}

	if unitsCount != 1 {
		t.Errorf("se esperaba 1 unidad de concepto, se obtuvieron %d", unitsCount)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("error leyendo paquete zip generado: %v", err)
	}

	files := make(map[string]bool)
	for _, f := range zipReader.File {
		files[f.Name] = true
	}

	if !files["index.md"] {
		t.Error("el bundle generado no contiene 'index.md'")
	}
	if !files["log.md"] {
		t.Error("el bundle generado no contiene 'log.md'")
	}
	if !files["documento.md"] {
		t.Error("el bundle breve generado debe contener 'documento.md'")
	}
}

func TestOKFConverter_MultipleChapters(t *testing.T) {
	conv := NewOKFConverter()
	jobID := uuid.New()
	content := []byte(`Lorem Ipsum
A Book in Fifteen Chapters

CHAPTER 1
The Beginning
Lorem ipsum dolor sit amet, consectetur adipiscing elit.

CHAPTER 2
A New Direction
Lorem ipsum dolor sit amet, consectetur adipiscing elit.

CHAPTER 3
Shifting Sands
Lorem ipsum dolor sit amet, consectetur adipiscing elit.`)

	zipBytes, unitsCount, err := conv.ConvertToOKFBundle(jobID, "book.pdf", content, []string{})
	if err != nil {
		t.Fatalf("error convirtiendo documento con capítulos: %v", err)
	}

	if unitsCount != 3 {
		t.Errorf("se esperaban 3 unidades de capítulos, se obtuvieron %d", unitsCount)
	}

	zipReader, _ := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	filesMap := make(map[string]*zip.File)
	for _, f := range zipReader.File {
		filesMap[f.Name] = f
	}

	if filesMap["capitulo-01.md"] == nil || filesMap["capitulo-02.md"] == nil || filesMap["capitulo-03.md"] == nil {
		t.Error("el bundle debe contener capitulo-01.md, capitulo-02.md y capitulo-03.md")
	}

	indexFile := filesMap["index.md"]
	rc, _ := indexFile.Open()
	indexContent, _ := io.ReadAll(rc)
	rc.Close()

	if !bytes.Contains(indexContent, []byte("Capítulo 1")) || !bytes.Contains(indexContent, []byte("capitulo-01.md")) {
		t.Errorf("index.md debe enlazar capitulo-01.md. Obtenido:\n%s", string(indexContent))
	}
}

func TestOKFConverter_LongDocumentAutoSegmentation(t *testing.T) {
	conv := NewOKFConverter()
	jobID := uuid.New()

	var sb strings.Builder
	for i := 1; i <= 30; i++ {
		sb.WriteString(fmt.Sprintf("Párrafo %d: Este es un texto largo explicativo que simula un documento extenso sin formato markdown explicito.\n\n", i))
	}

	zipBytes, unitsCount, err := conv.ConvertToOKFBundle(jobID, "documento_largo.txt", []byte(sb.String()), []string{})
	if err != nil {
		t.Fatalf("error convirtiendo documento largo: %v", err)
	}

	if unitsCount <= 1 {
		t.Errorf("se esperaban múltiples unidades lógicas para un documento extenso, pero se obtuvieron %d", unitsCount)
	}

	zipReader, _ := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	filesMap := make(map[string]*zip.File)
	for _, f := range zipReader.File {
		filesMap[f.Name] = f
	}

	indexFile := filesMap["index.md"]
	rc, _ := indexFile.Open()
	indexContent, _ := io.ReadAll(rc)
	rc.Close()

	if !bytes.Contains(indexContent, []byte("capitulo-01.md")) || !bytes.Contains(indexContent, []byte("capitulo-02.md")) {
		t.Errorf("index.md debe listar los capítulos segmentados. Obtenido:\n%s", string(indexContent))
	}
}

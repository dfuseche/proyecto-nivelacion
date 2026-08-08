package validator

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
)

type ValidationResult struct {
	IsValid  bool
	HasWarns bool
	Errors   []string
	Warnings []string
}

type OKFValidator struct{}

func NewOKFValidator() *OKFValidator {
	return &OKFValidator{}
}

// ValidateZipBundle comprueba las condiciones mínimas obligatorias del Bundle OKF antes de publicarlo
func (v *OKFValidator) ValidateZipBundle(zipData []byte) *ValidationResult {
	res := &ValidationResult{
		IsValid:  true,
		HasWarns: false,
		Errors:   []string{},
		Warnings: []string{},
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		res.IsValid = false
		res.Errors = append(res.Errors, "el paquete no es un archivo ZIP válido")
		return res
	}

	filesMap := make(map[string]*zip.File)
	for _, f := range zipReader.File {
		filesMap[f.Name] = f
	}

	// 1. Verificación obligatoria de index.md
	indexFile, hasIndex := filesMap["index.md"]
	if !hasIndex {
		res.IsValid = false
		res.Errors = append(res.Errors, "ausencia crítica de 'index.md' en la raíz del bundle")
	}

	// 2. Verificación obligatoria de log.md
	_, hasLog := filesMap["log.md"]
	if !hasLog {
		res.IsValid = false
		res.Errors = append(res.Errors, "ausencia crítica de 'log.md' en la raíz del bundle")
	}

	if !res.IsValid {
		return res
	}

	// 3. Verificación de hipervínculos dentro de index.md
	indexContentBytes, err := readZipFileContent(indexFile)
	if err != nil {
		res.IsValid = false
		res.Errors = append(res.Errors, "error al leer el contenido de index.md")
		return res
	}

	indexContent := string(indexContentBytes)
	// Regex para extraer enlaces Markdown [Texto](archivo.md)
	linkRegex := regexp.MustCompile(`\[.*?\]\((.*?\.md)\)`)
	matches := linkRegex.FindAllStringSubmatch(indexContent, -1)

	if len(matches) == 0 {
		res.HasWarns = true
		res.Warnings = append(res.Warnings, "el archivo index.md no contiene enlaces a documentos de concepto")
	}

	for _, match := range matches {
		if len(match) > 1 {
			targetFile := match[1]
			if _, exists := filesMap[targetFile]; !exists {
				res.IsValid = false
				res.Errors = append(res.Errors, fmt.Sprintf("enlace roto en index.md: el archivo '%s' no existe en el bundle", targetFile))
			}
		}
	}

	return res
}

func readZipFileContent(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

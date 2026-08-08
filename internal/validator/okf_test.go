package validator

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/google/uuid"

	"okf-converter/internal/converter"
)

func TestOKFValidator_ValidBundle(t *testing.T) {
	conv := converter.NewOKFConverter()
	val := NewOKFValidator()

	zipBytes, _, _ := conv.ConvertToOKFBundle(uuid.New(), "test.md", []byte("# Seccion 1\nContenido"), []string{})

	res := val.ValidateZipBundle(zipBytes)
	if !res.IsValid {
		t.Errorf("se esperaba bundle válido, pero falló con errores: %v", res.Errors)
	}
}

func TestOKFValidator_MissingIndexMD(t *testing.T) {
	val := NewOKFValidator()

	// Crear ZIP sin index.md
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("log.md")
	w.Write([]byte("log content"))
	w, _ = zw.Create("documento.md")
	w.Write([]byte("doc content"))
	zw.Close()

	res := val.ValidateZipBundle(buf.Bytes())
	if res.IsValid {
		t.Error("se esperaba que la validación fallara ante la falta de index.md")
	}
}

func TestOKFValidator_BrokenLink(t *testing.T) {
	val := NewOKFValidator()

	// Crear ZIP con index.md referenciando un archivo inexistente
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("index.md")
	w.Write([]byte("1. [Capitulo Roto](fantasma.md)"))
	w, _ = zw.Create("log.md")
	w.Write([]byte("log content"))
	zw.Close()

	res := val.ValidateZipBundle(buf.Bytes())
	if res.IsValid {
		t.Error("se esperaba que la validación fallara ante un enlace roto en index.md")
	}
}

package apitest

import (
	"bytes"
	"mime/multipart"
	"testing"
)

// Minimal valid file signatures for upload tests.
var (
	PNG  = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), make([]byte, 32)...)
	JPEG = append([]byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00"), make([]byte, 32)...)
	EPUB = append([]byte("PK\x03\x04\x0a\x00\x00\x00\x00\x00mimetypeapplication/epub+zip"), make([]byte, 32)...)
)

type File struct {
	Field, Name string
	Content     []byte
}

// Multipart builds a multipart/form-data body; returns (body, content-type).
func Multipart(t *testing.T, fields map[string]string, files ...File) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		fw, err := w.CreateFormFile(f.Field, f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(f.Content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String(), w.FormDataContentType()
}

// internal/handlers/http/admin_handler.go
package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type DocMeta struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

func AdminListDocs(w http.ResponseWriter, r *http.Request) {
	root := "uploads"
	_ = os.MkdirAll(root, 0755)

	files, _ := os.ReadDir(root)
	var list []DocMeta
	for _, f := range files {
		if f.IsDir() { continue }
		info, _ := f.Info()
		list = append(list, DocMeta{Filename: f.Name(), Size: info.Size()})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"docs": list})
}

func AdminUploadDoc(w http.ResponseWriter, r *http.Request) {
	root := "uploads"
	_ = os.MkdirAll(root, 0755)

	f, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file missing", http.StatusBadRequest)
		return
	}
	defer f.Close()

	dst := filepath.Join(root, filepath.Base(hdr.Filename))
	out, err := os.Create(dst)
	if err != nil {
		http.Error(w, "write error", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	n, _ := io.Copy(out, f)

	// TODO: panggil pipeline vectorize embeddings di background
	// (gunakan pkg/vector + cfg.LLM.EmbeddingModel)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"saved":"%s","bytes":%d}`, filepath.Base(dst), n)
}

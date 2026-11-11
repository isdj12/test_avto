package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"test_design/techniques"
)

//go:embed index.html
var static embed.FS

const templatesDir = "templates"

type generateReq struct {
	Technique string                 `json:"technique"`
	Filename  string                 `json:"filename"`
	Params    []techniques.Parameter `json:"params"`
	OR        techniques.ORConfig    `json:"or"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func cleanParams(in []techniques.Parameter) (out []techniques.Parameter) {
	for _, p := range in {
		name := strings.TrimSpace(p.Name)
		var vals []string
		for _, v := range p.Values {
			if v = strings.TrimSpace(v); v != "" {
				vals = append(vals, v)
			}
		}
		if name != "" && len(vals) > 0 {
			out = append(out, techniques.Parameter{Name: name, Values: vals})
		}
	}
	return out
}

func listTemplates(w http.ResponseWriter, r *http.Request) {
	files, _ := filepath.Glob(filepath.Join(templatesDir, "*.txt"))
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	sort.Strings(names)
	writeJSON(w, 200, names)
}

func getTemplate(w http.ResponseWriter, r *http.Request) {
	params, err := techniques.ReadTemplate(filepath.Join(templatesDir, filepath.Base(r.PathValue("name"))))
	if err != nil {
		fail(w, 404, "Шаблон не найден")
		return
	}
	writeJSON(w, 200, params)
}

func listExcel(w http.ResponseWriter, r *http.Request) {
	files, _ := filepath.Glob(filepath.Join(techniques.OutputDir, "*.xlsx"))
	type item struct {
		Name string `json:"name"`
		Time string `json:"time"`
	}
	items := []item{}
	sort.Slice(files, func(i, j int) bool {
		a, _ := os.Stat(files[i])
		b, _ := os.Stat(files[j])
		return a != nil && b != nil && a.ModTime().After(b.ModTime())
	})
	for _, f := range files {
		if st, err := os.Stat(f); err == nil {
			items = append(items, item{filepath.Base(f), st.ModTime().Format("02.01.2006 15:04")})
		}
	}
	writeJSON(w, 200, items)
}

func generate(w http.ResponseWriter, r *http.Request) {
	var req generateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, 400, "Некорректный запрос")
		return
	}
	params := cleanParams(req.Params)
	if len(params) == 0 {
		fail(w, 400, "Добавьте хотя бы один параметр со значениями")
		return
	}
	name := filepath.Base(strings.TrimSpace(req.Filename))
	if name == "." || name == "" || name == "/" {
		name = map[string]string{"pairwise": "Pairwise"}[req.Technique]
		if name == "" {
			name = "TPR"
		}
	}
	if req.OR.Label = strings.TrimSpace(req.OR.Label); req.OR.Label == "" {
		req.OR.Label = "Ожидаемый результат"
	}

	var (
		res *techniques.Result
		err error
	)
	switch req.Technique {
	case "tpr":
		res, err = techniques.GenerateTPR(params, req.OR, name)
	case "pairwise":
		res, err = techniques.GeneratePairwise(params, req.OR, name)
	default:
		fail(w, 400, "Выберите технику тест-дизайна")
		return
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	res.File = filepath.Base(res.File)
	writeJSON(w, 200, res)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "адрес панели")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/templates", listTemplates)
	mux.HandleFunc("GET /api/templates/{name}", getTemplate)
	mux.HandleFunc("GET /api/files", listExcel)
	mux.HandleFunc("POST /api/generate", generate)
	mux.HandleFunc("GET /excel/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.PathValue("name"))
		w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+strings.ReplaceAll(name, " ", "%20"))
		http.ServeFile(w, r, filepath.Join(techniques.OutputDir, name))
	})
	mux.Handle("GET /", http.FileServerFS(static))

	fmt.Printf("Панель запущена: http://%s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

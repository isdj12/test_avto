package pairwise

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Parameter struct {
	Name   string
	Values []string
}

type pair struct{ pi, vi, pj, vj int }

type ORConfig struct {
	Mode      int
	Label     string
	Postfix   string
	Threshold float64
	BelowText string
	AboveText string
	FixedText string
}

// ── Математика ────────────────────────────────────────────────

var numRx = regexp.MustCompile(`[+\-]?\d+(?:[.,]\d+)?`)
var exprRx = regexp.MustCompile(`([+\-\*/])(\d+(?:[.,]\d+)?)(%?)`)

func extractNum(s string) (float64, bool) {
	m := numRx.FindString(s)
	if m == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
	return v, err == nil
}

func sumCombo(params []Parameter, c []int) float64 {
	s := 0.0
	for i, idx := range c {
		if v, ok := extractNum(params[i].Values[idx]); ok {
			s += v
		}
	}
	return s
}

func productCombo(params []Parameter, c []int) float64 {
	p := 1.0
	for i, idx := range c {
		if v, ok := extractNum(params[i].Values[idx]); ok {
			p *= v
		}
	}
	return p
}

func smartCombo(params []Parameter, c []int) float64 {
	if len(c) == 0 {
		return 0
	}
	base, _ := extractNum(params[0].Values[c[0]])
	for i := 1; i < len(c); i++ {
		v := params[i].Values[c[i]]
		m := exprRx.FindStringSubmatch(v)
		if m == nil {
			if n, ok := extractNum(v); ok {
				base += n
			}
			continue
		}
		num, _ := strconv.ParseFloat(strings.ReplaceAll(m[2], ",", "."), 64)
		if m[3] == "%" {
			num = base * num / 100
		}
		switch m[1] {
		case "+":
			base += num
		case "-":
			base -= num
		case "*":
			base *= num
		case "/":
			if num != 0 {
				base /= num
			}
		}
	}
	return base
}

func joinNames(params []Parameter, c []int) string {
	parts := make([]string, len(c))
	for i, idx := range c {
		parts[i] = params[i].Values[idx]
	}
	return strings.Join(parts, " / ")
}

func fmtVal(v float64, pfx string) string {
	sfx := ""
	if pfx != "" {
		sfx = " " + pfx
	}
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f%s", v, sfx)
	}
	return fmt.Sprintf("%.2f%s", v, sfx)
}

func orValue(params []Parameter, c []int, cfg ORConfig) string {
	switch cfg.Mode {
	case 1:
		return fmtVal(sumCombo(params, c), cfg.Postfix)
	case 2:
		return fmtVal(smartCombo(params, c), cfg.Postfix)
	case 3:
		return fmtVal(productCombo(params, c), cfg.Postfix)
	case 4:
		if sumCombo(params, c) < cfg.Threshold {
			return cfg.BelowText
		}
		return cfg.AboveText
	case 5:
		return joinNames(params, c)
	case 6:
		return cfg.FixedText
	}
	return ""
}

func multStr(params []Parameter) string {
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = strconv.Itoa(len(p.Values))
	}
	return strings.Join(parts, " x ")
}

// ── Pairwise алгоритм ────────────────────────────────────────

func genPairwise(params []Parameter) [][]int {
	n := len(params)
	if n == 0 {
		return nil
	}
	if n == 1 {
		var r [][]int
		for i := range params[0].Values {
			r = append(r, []int{i})
		}
		return r
	}

	uncovered := map[pair]bool{}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for vi := range params[i].Values {
				for vj := range params[j].Values {
					uncovered[pair{i, vi, j, vj}] = true
				}
			}
		}
	}

	freq := make([][]int, n)
	for i := range freq {
		freq[i] = make([]int, len(params[i].Values))
	}

	var tests [][]int
	for len(uncovered) > 0 {
		best := make([]int, n)
		bestCount := -1
		bestFreq := math.MaxInt64
		cand := make([]int, n)

		var search func(int)
		search = func(idx int) {
			if idx == n {
				cnt := 0
				for i := 0; i < n; i++ {
					for j := i + 1; j < n; j++ {
						if uncovered[pair{i, cand[i], j, cand[j]}] {
							cnt++
						}
					}
				}
				fs := 0
				for i := 0; i < n; i++ {
					fs += freq[i][cand[i]]
				}
				if cnt > bestCount || (cnt == bestCount && fs < bestFreq) {
					bestCount = cnt
					bestFreq = fs
					copy(best, cand)
				}
				return
			}
			for vi := range params[idx].Values {
				cand[idx] = vi
				search(idx + 1)
			}
		}
		search(0)

		if bestCount <= 0 {
			break
		}

		tc := make([]int, n)
		copy(tc, best)
		tests = append(tests, tc)

		for i := 0; i < n; i++ {
			freq[i][best[i]]++
			for j := i + 1; j < n; j++ {
				delete(uncovered, pair{i, best[i], j, best[j]})
			}
		}
	}

	sort.Slice(tests, func(a, b int) bool {
		for k := 0; k < n; k++ {
			if tests[a][k] != tests[b][k] {
				return tests[a][k] < tests[b][k]
			}
		}
		return false
	})
	return tests
}

// ── Excel ─────────────────────────────────────────────────────

func colName(idx int) string {
	name := ""
	idx++
	for idx > 0 {
		idx--
		name = string(rune('A'+idx%26)) + name
		idx /= 26
	}
	return name
}

func ca(col, row int) string { return colName(col) + strconv.Itoa(row) }

func border() []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
}

type styles struct{ hdr, sec, cell, orCell, orHdr, plain, num int }

func mkStyles(f *excelize.File) styles {
	s := styles{}
	s.hdr, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Family: "Arial"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border(),
	})
	s.sec, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Family: "Arial"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    border(),
	})
	s.cell, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Family: "Arial"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border(),
	})
	s.orCell, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Family: "Arial"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border(),
	})
	s.orHdr, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Family: "Arial"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border:    border(),
	})
	s.plain, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Family: "Arial"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "top", WrapText: true},
	})
	s.num, _ = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Family: "Arial"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border(),
	})
	return s
}

// ── Вкладка: Попарное тестирование ──────────────────────────

func buildPairwise(f *excelize.File, params []Parameter, cfg ORConfig, st styles) ([][]int, int, int, float64) {
	const sn = "Попарное тестирование"
	f.NewSheet(sn)

	tests := genPairwise(params)
	n := len(params)
	nt := len(tests)

	totalTRP := 1
	for _, p := range params {
		totalTRP *= len(p.Values)
	}
	totalPairs := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			totalPairs += len(params[i].Values) * len(params[j].Values)
		}
	}
	covSet := map[pair]bool{}
	for _, tc := range tests {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				covSet[pair{i, tc[i], j, tc[j]}] = true
			}
		}
	}
	cov := len(covSet)
	reduction := (1.0 - float64(nt)/float64(totalTRP)) * 100.0

	// Шапка
	f.SetCellStr(sn, ca(0, 1), "№ теста")
	f.SetCellStyle(sn, ca(0, 1), ca(0, 1), st.hdr)
	for pi, p := range params {
		c := ca(pi+1, 1)
		f.SetCellStr(sn, c, fmt.Sprintf("П%d: %s (%d)", pi+1, p.Name, len(p.Values)))
		f.SetCellStyle(sn, c, c, st.hdr)
	}
	f.SetCellStr(sn, ca(n+1, 1), fmt.Sprintf("ОР: %s", cfg.Label))
	f.SetCellStyle(sn, ca(n+1, 1), ca(n+1, 1), st.hdr)
	f.SetRowHeight(sn, 1, 24)

	// Строки тестов
	for ti, tc := range tests {
		row := ti + 2
		f.SetCellStr(sn, ca(0, row), strconv.Itoa(ti+1))
		f.SetCellStyle(sn, ca(0, row), ca(0, row), st.num)
		for pi := range params {
			c := ca(pi+1, row)
			f.SetCellStr(sn, c, params[pi].Values[tc[pi]])
			f.SetCellStyle(sn, c, c, st.cell)
		}
		orC := ca(n+1, row)
		if cfg.Mode > 0 {
			f.SetCellStr(sn, orC, orValue(params, tc, cfg))
		}
		f.SetCellStyle(sn, orC, orC, st.orCell)
		f.SetRowHeight(sn, row, 36)
	}

	// Статистика
	sr := nt + 3
	f.SetCellStr(sn, ca(0, sr), fmt.Sprintf(
		"Пар: %d  |  Покрыто: %d (%.0f%%)  |  Pairwise: %d  |  TPR: %d  |  Сокращение: %.1f%%",
		totalPairs, cov, float64(cov)/float64(totalPairs)*100, nt, totalTRP, reduction))
	f.SetCellStyle(sn, ca(0, sr), ca(0, sr), st.plain)
	f.SetRowHeight(sn, sr, 16)

	// Ширины
	f.SetColWidth(sn, "A", "A", 9)
	for pi, p := range params {
		maxW := len([]rune(p.Name)) + 8
		for _, v := range p.Values {
			if l := len([]rune(v)) + 3; l > maxW {
				maxW = l
			}
		}
		w := float64(maxW) * 0.65
		if w < 14 {
			w = 14
		}
		if w > 36 {
			w = 36
		}
		f.SetColWidth(sn, colName(pi+1), colName(pi+1), w)
	}
	f.SetColWidth(sn, colName(n+1), colName(n+1), 26)

	return tests, totalTRP, totalPairs, reduction
}

// ── Вкладка: Чек-лист ────────────────────────────────────────

func buildChecklist(f *excelize.File, params []Parameter, tests [][]int, cfg ORConfig, st styles) {
	const sn = "Чек-лист"
	f.NewSheet(sn)
	n := len(params)

	for i, h := range []string{"#", "Параметры", "Входные данные", fmt.Sprintf("ОР: %s", cfg.Label), "Статус", "Комментарий"} {
		c := ca(i, 1)
		f.SetCellStr(sn, c, h)
		f.SetCellStyle(sn, c, c, st.hdr)
	}
	f.SetRowHeight(sn, 1, 22)

	for ti, tc := range tests {
		row := ti + 2
		f.SetCellStr(sn, ca(0, row), strconv.Itoa(ti+1))
		f.SetCellStyle(sn, ca(0, row), ca(0, row), st.num)

		parts := make([]string, n)
		for pi := range params {
			parts[pi] = fmt.Sprintf("П%d=%s", pi+1, params[pi].Values[tc[pi]])
		}
		f.SetCellStr(sn, ca(1, row), strings.Join(parts, "; "))
		f.SetCellStyle(sn, ca(1, row), ca(1, row), st.cell)

		steps := make([]string, n)
		for pi := range params {
			steps[pi] = fmt.Sprintf("%s: %s", params[pi].Name, params[pi].Values[tc[pi]])
		}
		f.SetCellStr(sn, ca(2, row), strings.Join(steps, "\n"))
		f.SetCellStyle(sn, ca(2, row), ca(2, row), st.cell)

		if cfg.Mode > 0 {
			f.SetCellStr(sn, ca(3, row), orValue(params, tc, cfg))
		}
		f.SetCellStyle(sn, ca(3, row), ca(3, row), st.orCell)
		f.SetCellStyle(sn, ca(4, row), ca(4, row), st.cell)
		f.SetCellStyle(sn, ca(5, row), ca(5, row), st.cell)
		f.SetRowHeight(sn, row, 38)
	}

	f.SetColWidth(sn, "A", "A", 6)
	f.SetColWidth(sn, "B", "B", 38)
	f.SetColWidth(sn, "C", "C", 42)
	f.SetColWidth(sn, "D", "D", 24)
	f.SetColWidth(sn, "E", "E", 14)
	f.SetColWidth(sn, "F", "F", 30)
}

// ── Вкладка: Вывод по двум работам ──────────────────────────

func buildSummary(f *excelize.File, params []Parameter, tests [][]int, totalTRP, totalPairs int, reduction float64, cfg ORConfig, st styles) {
	const sn = "Вывод по двум работам"
	f.NewSheet(sn)
	nt := len(tests)

	orDesc := map[int]string{
		1: "Сумма числовых значений",
		2: "Умный расчёт (базовый + операции)",
		3: "Произведение числовых значений",
		5: "Текстовое имя из значений",
	}
	modeStr := "Не заполняется"
	if d, ok := orDesc[cfg.Mode]; ok {
		modeStr = d
		if cfg.Postfix != "" {
			modeStr += " [" + cfg.Postfix + "]"
		}
	} else if cfg.Mode == 4 {
		modeStr = fmt.Sprintf("Порог %.2f: «%s» / «%s»", cfg.Threshold, cfg.BelowText, cfg.AboveText)
	} else if cfg.Mode == 6 {
		if cfg.FixedText == "" {
			modeStr = "Ячейки пустые"
		} else {
			modeStr = "Фикс: «" + cfg.FixedText + "»"
		}
	}

	rows := []struct{ l, v string }{
		{"Метод 1", "Таблица принятия решений (TPR / Decision Table)"},
		{"Метод 2", "Попарное тестирование (Pairwise / All-Pairs)"},
		{"Параметров", strconv.Itoa(len(params))},
		{"Значений", multStr(params)},
		{"", ""},
		{"TPR — тест-кейсов", fmt.Sprintf("%d (полный перебор)", totalTRP)},
		{"Pairwise — тест-кейсов", fmt.Sprintf("%d (все пары покрыты)", nt)},
		{"Сокращение", fmt.Sprintf("%.1f%%", reduction)},
		{"Пар для покрытия", fmt.Sprintf("%d (покрыто 100%%)", totalPairs)},
		{"", ""},
		{"Поле ОР", cfg.Label},
		{"Режим ОР", modeStr},
		{"", ""},
		{"Вывод", fmt.Sprintf(
			"Pairwise сокращает тест-кейсы с %d до %d (на %.1f%%), "+
				"сохраняя 100%% покрытие всех %d попарных комбинаций. "+
				"TPR гарантирует полное покрытие, но трудозатратен при большом числе параметров.",
			totalTRP, nt, reduction, totalPairs)},
	}

	for i, r := range rows {
		row := i + 1
		if r.l == "" {
			f.SetRowHeight(sn, row, 8)
			continue
		}
		f.SetCellStr(sn, ca(0, row), r.l)
		f.SetCellStyle(sn, ca(0, row), ca(0, row), st.sec)
		f.MergeCell(sn, ca(1, row), ca(3, row))
		f.SetCellStr(sn, ca(1, row), r.v)
		f.SetCellStyle(sn, ca(1, row), ca(3, row), st.cell)
		h := 20.0
		if r.l == "Вывод" {
			h = 80
		}
		f.SetRowHeight(sn, row, h)
	}
	f.SetColWidth(sn, "A", "A", 24)
	f.SetColWidth(sn, "B", "D", 38)
}

// ── Чтение файла ─────────────────────────────────────────────

func readFile(filename string) ([]Parameter, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var params []Parameter
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		var vals []string
		for _, v := range strings.Split(parts[1], ",") {
			if v = strings.TrimSpace(v); v != "" {
				vals = append(vals, v)
			}
		}
		if name != "" && len(vals) > 0 {
			params = append(params, Parameter{Name: name, Values: vals})
		}
	}
	return params, sc.Err()
}

func addXlsx(name string) string {
	if !strings.HasSuffix(strings.ToLower(name), ".xlsx") {
		return name + ".xlsx"
	}
	return name
}

// OutputDir — папка для Excel-файлов, если в имени выходного файла не указан путь.
var OutputDir = "excel"

// Result — итог генерации pairwise.
type Result struct {
	File      string
	TPR       int
	Pairwise  int
	Pairs     int
	Reduction float64
}

// Generate строит pairwise + чек-лист + вывод и сохраняет в outputFile (.xlsx).
func Generate(params []Parameter, cfg ORConfig, outputFile string) (*Result, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("не заданы параметры")
	}
	outputFile = addXlsx(outputFile)
	if filepath.Dir(outputFile) == "." {
		outputFile = filepath.Join(OutputDir, outputFile)
	}
	if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
		return nil, err
	}
	total := 1
	for _, p := range params {
		total *= len(p.Values)
	}
	f := excelize.NewFile()
	defer f.Close()
	st := mkStyles(f)
	tests, _, totalPairs, reduction := buildPairwise(f, params, cfg, st)
	buildChecklist(f, params, tests, cfg, st)
	buildSummary(f, params, tests, total, totalPairs, reduction, cfg, st)
	f.DeleteSheet("Sheet1")
	if err := f.SaveAs(outputFile); err != nil {
		return nil, err
	}
	return &Result{File: outputFile, TPR: total, Pairwise: len(tests), Pairs: totalPairs, Reduction: reduction}, nil
}

// ReadTemplate читает файл шаблона (Имя: Значение1, Значение2, ...).
func ReadTemplate(filename string) ([]Parameter, error) { return readFile(filename) }

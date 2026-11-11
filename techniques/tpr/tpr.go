package tpr

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Parameter struct {
	Name   string
	Values []string
}

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

func cartesian(params []Parameter) [][]int {
	res := [][]int{{}}
	for _, p := range params {
		var next [][]int
		for _, combo := range res {
			for i := range p.Values {
				nc := make([]int, len(combo)+1)
				copy(nc, combo)
				nc[len(combo)] = i
				next = append(next, nc)
			}
		}
		res = next
	}
	return res
}

func multStr(params []Parameter) string {
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = strconv.Itoa(len(p.Values))
	}
	return strings.Join(parts, " x ")
}

func pluralVal(n int) string {
	if n%10 == 1 && n%100 != 11 {
		return "значение"
	}
	if n%10 >= 2 && n%10 <= 4 && (n%100 < 10 || n%100 >= 20) {
		return "значения"
	}
	return "значений"
}

// ── Excel ────────────────────────────────────────────────────

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

type styles struct{ hdr, sec, cell, orCell, orHdr, plain int }

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
	return s
}

// ── Построение ТПР ──────────────────────────────────────────

func buildTPR(f *excelize.File, params []Parameter, cfg ORConfig, st styles) {
	const sn = "Таблица принятия решений"
	f.NewSheet(sn)

	combos := cartesian(params)
	n := len(params)
	nr := len(combos)

	// Строка 1: Правила | R1 | R2 | ...
	f.SetCellStr(sn, ca(0, 1), "Правила")
	f.SetCellStyle(sn, ca(0, 1), ca(0, 1), st.hdr)
	for i := 0; i < nr; i++ {
		c := ca(i+1, 1)
		f.SetCellStr(sn, c, fmt.Sprintf("R%d", i+1))
		f.SetCellStyle(sn, c, c, st.hdr)
	}
	f.SetRowHeight(sn, 1, 22)

	// Строка 2: Условия (объединённая)
	f.MergeCell(sn, ca(0, 2), ca(nr, 2))
	f.SetCellStr(sn, ca(0, 2), "Условия")
	f.SetCellStyle(sn, ca(0, 2), ca(nr, 2), st.sec)
	f.SetRowHeight(sn, 2, 18)

	// Строки параметров
	for pi, p := range params {
		row := pi + 3
		f.SetCellStr(sn, ca(0, row), fmt.Sprintf("%s (%d %s)", p.Name, len(p.Values), pluralVal(len(p.Values))))
		f.SetCellStyle(sn, ca(0, row), ca(0, row), st.orHdr)
		for ci, combo := range combos {
			c := ca(ci+1, row)
			f.SetCellStr(sn, c, p.Values[combo[pi]])
			f.SetCellStyle(sn, c, c, st.cell)
		}
		f.SetRowHeight(sn, row, 40)
	}

	// Строка «Результат (ОР)» (объединённая)
	secRow := n + 3
	f.MergeCell(sn, ca(0, secRow), ca(nr, secRow))
	f.SetCellStr(sn, ca(0, secRow), "Результат (ОР)")
	f.SetCellStyle(sn, ca(0, secRow), ca(nr, secRow), st.sec)
	f.SetRowHeight(sn, secRow, 18)

	// Строка значений ОР
	orRow := n + 4
	f.SetCellStr(sn, ca(0, orRow), cfg.Label)
	f.SetCellStyle(sn, ca(0, orRow), ca(0, orRow), st.orHdr)
	for ci, combo := range combos {
		c := ca(ci+1, orRow)
		if cfg.Mode > 0 {
			f.SetCellStr(sn, c, orValue(params, combo, cfg))
		}
		f.SetCellStyle(sn, c, c, st.orCell)
	}
	f.SetRowHeight(sn, orRow, 24)

	// Пояснения
	notes := []string{
		"Пояснение структуры ТПР:",
		"1. Таблица принятия решений — метод тест-дизайна для полного перебора всех комбинаций.",
		fmt.Sprintf("2. Строки = условия (%d параметра(ов)), столбцы R1..R%d = правила (комбинации).", n, nr),
		"3. Строка «Результат (ОР)» — ожидаемый результат для каждого правила.",
		fmt.Sprintf("4. Итого правил: %d = %s", nr, multStr(params)),
	}
	for i, note := range notes {
		row := orRow + 2 + i
		f.SetCellStr(sn, ca(0, row), note)
		f.SetCellStyle(sn, ca(0, row), ca(0, row), st.plain)
		f.SetRowHeight(sn, row, 16)
	}

	// Ширины
	maxA := 22
	for _, p := range params {
		if l := len([]rune(p.Name)) + 16; l > maxA {
			maxA = l
		}
	}
	colAW := float64(maxA) * 0.6
	if colAW < 22 {
		colAW = 22
	}
	if colAW > 45 {
		colAW = 45
	}
	f.SetColWidth(sn, "A", "A", colAW)

	maxV := 0
	for _, p := range params {
		for _, v := range p.Values {
			if l := len([]rune(v)); l > maxV {
				maxV = l
			}
		}
	}
	colW := float64(maxV)*0.65 + 2
	if colW < 10 {
		colW = 10
	}
	if colW > 28 {
		colW = 28
	}
	if nr > 0 {
		f.SetColWidth(sn, "B", colName(nr), colW)
	}
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

// Result — итог генерации ТПР.
type Result struct {
	File  string
	Total int
}

// Generate строит ТПР по параметрам и сохраняет в outputFile (.xlsx).
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
	buildTPR(f, params, cfg, mkStyles(f))
	f.DeleteSheet("Sheet1")
	if err := f.SaveAs(outputFile); err != nil {
		return nil, err
	}
	return &Result{File: outputFile, Total: total}, nil
}

// ReadTemplate читает файл шаблона (Имя: Значение1, Значение2, ...).
func ReadTemplate(filename string) ([]Parameter, error) { return readFile(filename) }

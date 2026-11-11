package techniques

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

var OutputDir = "excel"

type Parameter struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type ORConfig struct {
	Mode      int     `json:"mode"`
	Label     string  `json:"label"`
	Postfix   string  `json:"postfix"`
	Threshold float64 `json:"threshold"`
	BelowText string  `json:"below"`
	AboveText string  `json:"above"`
	FixedText string  `json:"fixed"`
}

type Result struct {
	File      string  `json:"file"`
	Total     int     `json:"total"`
	Pairwise  int     `json:"pairwise,omitempty"`
	Pairs     int     `json:"pairs,omitempty"`
	Reduction float64 `json:"reduction,omitempty"`
}

var (
	numRx   = regexp.MustCompile(`[+\-]?\d+(?:[.,]\d+)?`)
	exprRx  = regexp.MustCompile(`([+\-\*/])(\d+(?:[.,]\d+)?)(%?)`)
	priceRx = regexp.MustCompile(`\(([^()]*)\)\s*$`)
)

func priceOf(s string) string {
	if m := priceRx.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return s
}

func extractNum(s string) (float64, bool) {
	m := numRx.FindString(priceOf(s))
	if m == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
	return v, err == nil
}

func sumCombo(params []Parameter, c []int) (s float64) {
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
		m := exprRx.FindStringSubmatch(priceOf(v))
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
		parts := make([]string, len(c))
		for i, idx := range c {
			parts[i] = params[i].Values[idx]
		}
		return strings.Join(parts, " / ")
	case 6:
		return cfg.FixedText
	}
	return ""
}

func totalCombos(params []Parameter) int {
	t := 1
	for _, p := range params {
		t *= len(p.Values)
	}
	return t
}

func multStr(params []Parameter) string {
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = strconv.Itoa(len(p.Values))
	}
	return strings.Join(parts, " x ")
}

func colName(idx int) string {
	name, _ := excelize.ColumnNumberToName(idx + 1)
	return name
}

func ca(col, row int) string {
	ref, _ := excelize.CoordinatesToCellName(col+1, row)
	return ref
}

type styles struct{ hdr, sec, cell, orCell, orHdr, plain, num int }

func mkStyles(f *excelize.File) styles {
	b := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
	mk := func(bold bool, h, v string, wrap, brd bool) int {
		st := &excelize.Style{
			Font:      &excelize.Font{Bold: bold, Size: 10, Family: "Arial"},
			Alignment: &excelize.Alignment{Horizontal: h, Vertical: v, WrapText: wrap},
		}
		if brd {
			st.Border = b
		}
		id, _ := f.NewStyle(st)
		return id
	}
	return styles{
		hdr:    mk(true, "center", "center", true, true),
		sec:    mk(true, "left", "center", false, true),
		cell:   mk(false, "center", "center", true, true),
		orCell: mk(true, "center", "center", true, true),
		orHdr:  mk(true, "left", "center", true, true),
		plain:  mk(false, "left", "top", true, false),
		num:    mk(true, "center", "center", false, true),
	}
}

func setCell(f *excelize.File, sn, ref, val string, style int) {
	f.SetCellStr(sn, ref, val)
	f.SetCellStyle(sn, ref, ref, style)
}

func setMerged(f *excelize.File, sn, c1, c2, val string, style int) {
	f.MergeCell(sn, c1, c2)
	f.SetCellStr(sn, c1, val)
	f.SetCellStyle(sn, c1, c2, style)
}

func saveBook(f *excelize.File, outputFile string) (string, error) {
	if !strings.HasSuffix(strings.ToLower(outputFile), ".xlsx") {
		outputFile += ".xlsx"
	}
	if filepath.Dir(outputFile) == "." {
		outputFile = filepath.Join(OutputDir, outputFile)
	}
	if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
		return "", err
	}
	f.DeleteSheet("Sheet1")
	return outputFile, f.SaveAs(outputFile)
}

func ReadTemplate(filename string) ([]Parameter, error) {
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
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		var vals []string
		for _, v := range strings.Split(rest, ",") {
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

func cartesian(params []Parameter) [][]int {
	res := [][]int{{}}
	for _, p := range params {
		var next [][]int
		for _, combo := range res {
			for i := range p.Values {
				next = append(next, append(append([]int(nil), combo...), i))
			}
		}
		res = next
	}
	return res
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

func buildTPR(f *excelize.File, params []Parameter, cfg ORConfig, st styles) {
	const sn = "Таблица принятия решений"
	f.NewSheet(sn)

	combos := cartesian(params)
	n, nr := len(params), len(combos)

	setCell(f, sn, ca(0, 1), "Правила", st.hdr)
	for i := 0; i < nr; i++ {
		setCell(f, sn, ca(i+1, 1), fmt.Sprintf("R%d", i+1), st.hdr)
	}
	f.SetRowHeight(sn, 1, 22)

	setMerged(f, sn, ca(0, 2), ca(nr, 2), "Условия", st.sec)
	f.SetRowHeight(sn, 2, 18)

	for pi, p := range params {
		row := pi + 3
		setCell(f, sn, ca(0, row), fmt.Sprintf("%s (%d %s)", p.Name, len(p.Values), pluralVal(len(p.Values))), st.orHdr)
		for ci, combo := range combos {
			setCell(f, sn, ca(ci+1, row), p.Values[combo[pi]], st.cell)
		}
		f.SetRowHeight(sn, row, 40)
	}

	secRow, orRow := n+3, n+4
	setMerged(f, sn, ca(0, secRow), ca(nr, secRow), "Результат (ОР)", st.sec)
	f.SetRowHeight(sn, secRow, 18)

	setCell(f, sn, ca(0, orRow), cfg.Label, st.orHdr)
	for ci, combo := range combos {
		setCell(f, sn, ca(ci+1, orRow), orValue(params, combo, cfg), st.orCell)
	}
	f.SetRowHeight(sn, orRow, 24)

	notes := []string{
		"Пояснение структуры ТПР:",
		"1. Таблица принятия решений — метод тест-дизайна для полного перебора всех комбинаций.",
		fmt.Sprintf("2. Строки = условия (%d параметра(ов)), столбцы R1..R%d = правила (комбинации).", n, nr),
		"3. Строка «Результат (ОР)» — ожидаемый результат для каждого правила.",
		fmt.Sprintf("4. Итого правил: %d = %s", nr, multStr(params)),
	}
	for i, note := range notes {
		row := orRow + 2 + i
		setCell(f, sn, ca(0, row), note, st.plain)
		f.SetRowHeight(sn, row, 16)
	}

	maxA, maxV := 22, 0
	for _, p := range params {
		maxA = max(maxA, len([]rune(p.Name))+16)
		for _, v := range p.Values {
			maxV = max(maxV, len([]rune(v)))
		}
	}
	f.SetColWidth(sn, "A", "A", min(max(float64(maxA)*0.6, 22), 45))
	if nr > 0 {
		f.SetColWidth(sn, "B", colName(nr), min(max(float64(maxV)*0.65+2, 10), 28))
	}
}

func GenerateTPR(params []Parameter, cfg ORConfig, outputFile string) (*Result, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("не заданы параметры")
	}
	f := excelize.NewFile()
	defer f.Close()
	buildTPR(f, params, cfg, mkStyles(f))
	path, err := saveBook(f, outputFile)
	if err != nil {
		return nil, err
	}
	return &Result{File: path, Total: totalCombos(params)}, nil
}

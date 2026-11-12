package techniques

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

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

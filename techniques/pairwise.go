package techniques

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type pair struct{ pi, vi, pj, vj int }

func genPairwise(params []Parameter) [][]int {
	n := len(params)
	if n == 0 {
		return nil
	}
	if n == 1 {
		r := make([][]int, len(params[0].Values))
		for i := range r {
			r[i] = []int{i}
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
		best, cand := make([]int, n), make([]int, n)
		bestCount, bestFreq := -1, math.MaxInt64

		var search func(int)
		search = func(idx int) {
			if idx == n {
				cnt, fs := 0, 0
				for i := 0; i < n; i++ {
					fs += freq[i][cand[i]]
					for j := i + 1; j < n; j++ {
						if uncovered[pair{i, cand[i], j, cand[j]}] {
							cnt++
						}
					}
				}
				if cnt > bestCount || (cnt == bestCount && fs < bestFreq) {
					bestCount, bestFreq = cnt, fs
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

		tests = append(tests, append([]int(nil), best...))
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

func buildPairwise(f *excelize.File, params []Parameter, cfg ORConfig, st styles) ([][]int, int, float64) {
	const sn = "Попарное тестирование"
	f.NewSheet(sn)

	tests := genPairwise(params)
	n, nt := len(params), len(tests)
	totalTRP := totalCombos(params)

	totalPairs := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			totalPairs += len(params[i].Values) * len(params[j].Values)
		}
	}
	reduction := (1.0 - float64(nt)/float64(totalTRP)) * 100.0

	setCell(f, sn, ca(0, 1), "№ теста", st.hdr)
	for pi, p := range params {
		setCell(f, sn, ca(pi+1, 1), fmt.Sprintf("П%d: %s (%d)", pi+1, p.Name, len(p.Values)), st.hdr)
	}
	setCell(f, sn, ca(n+1, 1), "ОР: "+cfg.Label, st.hdr)
	f.SetRowHeight(sn, 1, 24)

	for ti, tc := range tests {
		row := ti + 2
		setCell(f, sn, ca(0, row), strconv.Itoa(ti+1), st.num)
		for pi := range params {
			setCell(f, sn, ca(pi+1, row), params[pi].Values[tc[pi]], st.cell)
		}
		setCell(f, sn, ca(n+1, row), orValue(params, tc, cfg), st.orCell)
		f.SetRowHeight(sn, row, 36)
	}

	sr := nt + 3
	setCell(f, sn, ca(0, sr), fmt.Sprintf(
		"Пар: %d  |  Покрыто: %d (100%%)  |  Pairwise: %d  |  TPR: %d  |  Сокращение: %.1f%%",
		totalPairs, totalPairs, nt, totalTRP, reduction), st.plain)
	f.SetRowHeight(sn, sr, 16)

	f.SetColWidth(sn, "A", "A", 9)
	for pi, p := range params {
		maxW := len([]rune(p.Name)) + 8
		for _, v := range p.Values {
			maxW = max(maxW, len([]rune(v))+3)
		}
		f.SetColWidth(sn, colName(pi+1), colName(pi+1), min(max(float64(maxW)*0.65, 14), 36))
	}
	f.SetColWidth(sn, colName(n+1), colName(n+1), 26)
	return tests, totalPairs, reduction
}

func buildChecklist(f *excelize.File, params []Parameter, tests [][]int, cfg ORConfig, st styles) {
	const sn = "Чек-лист"
	f.NewSheet(sn)
	n := len(params)

	for i, h := range []string{"#", "Параметры", "Входные данные", "ОР: " + cfg.Label, "Статус", "Комментарий"} {
		setCell(f, sn, ca(i, 1), h, st.hdr)
	}
	f.SetRowHeight(sn, 1, 22)

	for ti, tc := range tests {
		row := ti + 2
		setCell(f, sn, ca(0, row), strconv.Itoa(ti+1), st.num)

		parts, steps := make([]string, n), make([]string, n)
		for pi := range params {
			parts[pi] = fmt.Sprintf("П%d=%s", pi+1, params[pi].Values[tc[pi]])
			steps[pi] = fmt.Sprintf("%s: %s", params[pi].Name, params[pi].Values[tc[pi]])
		}
		setCell(f, sn, ca(1, row), strings.Join(parts, "; "), st.cell)
		setCell(f, sn, ca(2, row), strings.Join(steps, "\n"), st.cell)
		setCell(f, sn, ca(3, row), orValue(params, tc, cfg), st.orCell)
		setCell(f, sn, ca(4, row), "", st.cell)
		setCell(f, sn, ca(5, row), "", st.cell)
		f.SetRowHeight(sn, row, 38)
	}

	for col, w := range map[string]float64{"A": 6, "B": 38, "C": 42, "D": 24, "E": 14, "F": 30} {
		f.SetColWidth(sn, col, col, w)
	}
}

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
		setCell(f, sn, ca(0, row), r.l, st.sec)
		setMerged(f, sn, ca(1, row), ca(3, row), r.v, st.cell)
		h := 20.0
		if r.l == "Вывод" {
			h = 80
		}
		f.SetRowHeight(sn, row, h)
	}
	f.SetColWidth(sn, "A", "A", 24)
	f.SetColWidth(sn, "B", "D", 38)
}

func GeneratePairwise(params []Parameter, cfg ORConfig, outputFile string) (*Result, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("не заданы параметры")
	}
	total := totalCombos(params)
	f := excelize.NewFile()
	defer f.Close()
	st := mkStyles(f)
	tests, totalPairs, reduction := buildPairwise(f, params, cfg, st)
	buildChecklist(f, params, tests, cfg, st)
	buildSummary(f, params, tests, total, totalPairs, reduction, cfg, st)
	path, err := saveBook(f, outputFile)
	if err != nil {
		return nil, err
	}
	return &Result{File: path, Total: total, Pairwise: len(tests), Pairs: totalPairs, Reduction: reduction}, nil
}

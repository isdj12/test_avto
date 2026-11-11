package main

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

const defaultTemplateContent = `# Шаблон: Интернет-магазин настольных игр
# Формат: Имя параметра: Значение1, Значение2, ...
# Строки с # — комментарии, игнорируются
# Числа в скобках: (500) — база, (+200) — плюс, (-50) — минус

Игра: Взрывные котята (500), Бункер (700), Имаджинариум (900), Диксит (1200), Каркассон (1500)
Версия: Детская (+200), Дорожная (+0), Настольная (+500)
Комплект: Да (+150), Нет (+0)
Получение: Самовывоз (+0), Доставка (+100)
`

// ── Выбор шаблона ────────────────────────────────────────────

func findTemplates() []string {
	var found []string
	seen := map[string]bool{}
	// В папке templates/ — ВСЕ .txt файлы
	if matches, _ := filepath.Glob("templates/*.txt"); len(matches) > 0 {
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				found = append(found, m)
			}
		}
	}
	// В текущей папке — только input*.txt (запасной вариант)
	if matches, _ := filepath.Glob("input*.txt"); len(matches) > 0 {
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				found = append(found, m)
			}
		}
	}
	return found
}

func ensureDefaultTemplate() string {
	const name = "templates/input_boardgames.txt"
	os.MkdirAll("templates", 0755)
	_ = os.WriteFile(name, []byte(defaultTemplateContent), 0644)
	fmt.Printf("  Создан шаблон: %s\n", name)
	return name
}

func chooseTemplate(reader *bufio.Reader) string {
	templates := findTemplates()
	if len(templates) == 0 {
		fmt.Println("  Шаблоны input*.txt не найдены. Создаю дефолтный...")
		p := ensureDefaultTemplate()
		templates = []string{p}
	}

	fmt.Println()
	fmt.Println("  ┌─ Доступные шаблоны ──────────────────────────────────")
	for i, t := range templates {
		fmt.Printf("  │  [%d] %s\n", i+1, t)
	}
	fmt.Println("  └──────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  Номер, путь к файлу или Enter [%s]: ", templates[0])

	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return templates[0]
	}
	if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(templates) {
		return templates[n-1]
	}
	return line
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

func hasNumbers(params []Parameter) bool {
	for _, p := range params {
		for _, v := range p.Values {
			if numRx.MatchString(v) {
				return true
			}
		}
	}
	return false
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

func readManual(reader *bufio.Reader) []Parameter {
	var params []Parameter
	fmt.Println("  Формат: Имя: Значение1, Значение2, ...")
	fmt.Println("  Пустая строка — завершить.")
	fmt.Println()
	for {
		fmt.Print("  Параметр: ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			fmt.Println("  Нужно двоеточие: Имя: Значение1, Значение2")
			continue
		}
		name := strings.TrimSpace(parts[0])
		var vals []string
		for _, v := range strings.Split(parts[1], ",") {
			if v = strings.TrimSpace(v); v != "" {
				vals = append(vals, v)
			}
		}
		if name == "" || len(vals) == 0 {
			fmt.Println("  Имя и хотя бы одно значение обязательны.")
			continue
		}
		params = append(params, Parameter{Name: name, Values: vals})
		fmt.Printf("  ✓ П%d: %s → [%s]\n\n", len(params), name, strings.Join(vals, " | "))
	}
	return params
}

func ask(reader *bufio.Reader, msg string) string {
	fmt.Print(msg)
	s, _ := reader.ReadString('\n')
	return strings.TrimSpace(s)
}

func addXlsx(name string) string {
	if !strings.HasSuffix(strings.ToLower(name), ".xlsx") {
		return name + ".xlsx"
	}
	return name
}

func askOR(reader *bufio.Reader, params []Parameter) ORConfig {
	cfg := ORConfig{}
	ans := ask(reader, "Название поля ОР [Стоимость заказа]: ")
	if ans == "" {
		cfg.Label = "Стоимость заказа"
	} else {
		cfg.Label = ans
	}

	fmt.Println()
	if hasNumbers(params) {
		fmt.Println("  Обнаружены числовые значения.")
	}
	fmt.Println("  Режим расчёта ОР:")
	fmt.Println("  [1] Сумма чисел")
	fmt.Println("  [2] Умный расчёт (+100, -50, *1.5, +10%)")
	fmt.Println("  [3] Произведение чисел")
	fmt.Println("  [4] Порог: если сумма < X → текст A, иначе → текст B")
	fmt.Println("  [5] Текстовое имя из значений параметров")
	fmt.Println("  [6] Фиксированный текст / пустые ячейки")
	fmt.Println("  [0] Не заполнять ОР")
	fmt.Println()

	mode, _ := strconv.Atoi(ask(reader, "  Ваш выбор [0]: "))
	cfg.Mode = mode

	switch mode {
	case 1, 2, 3:
		cfg.Postfix = ask(reader, "  Постфикс (₽, руб., $ — или пусто): ")
	case 4:
		t, _ := strconv.ParseFloat(strings.ReplaceAll(ask(reader, "  Порог (число): "), ",", "."), 64)
		cfg.Threshold = t
		cfg.BelowText = ask(reader, "  Текст если < порога: ")
		cfg.AboveText = ask(reader, "  Текст если >= порога: ")
	case 6:
		cfg.FixedText = ask(reader, "  Текст (пусто = пустые ячейки): ")
	}
	fmt.Println()
	return cfg
}

// ── main ─────────────────────────────────────────────────────

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════")
	fmt.Println("  TPR — Таблица принятия решений")
	fmt.Println("══════════════════════════════════════════════════════")
	fmt.Println()

	outAns := ask(reader, "Имя выходного файла [TPR.xlsx]: ")
	outputFile := "TPR.xlsx"
	if outAns != "" {
		outputFile = addXlsx(outAns)
	}
	fmt.Println()

	fmt.Println("─── Источник данных ──────────────────────────────────")
	fmt.Println("  [F] Файл шаблона из templates/")
	fmt.Println("  [M] Ввести вручную")
	fmt.Println()

	var params []Parameter
	if src := strings.ToLower(ask(reader, "  Выбор [F]: ")); src == "m" || src == "м" {
		fmt.Println()
		params = readManual(reader)
	} else {
		file := chooseTemplate(reader)
		fmt.Println()
		var err error
		params, err = readFile(file)
		if err != nil {
			fmt.Printf("  [!] Ошибка: %v\n  Переключаюсь на ручной ввод...\n\n", err)
			params = readManual(reader)
		} else {
			fmt.Printf("  Загружено параметров: %d из «%s»\n", len(params), file)
		}
	}

	if len(params) == 0 {
		fmt.Println("Параметры не введены. Выход.")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("─── Параметры ────────────────────────────────────────")
	total := 1
	for i, p := range params {
		fmt.Printf("  П%d: %s → %d %s\n", i+1, p.Name, len(p.Values), pluralVal(len(p.Values)))
		total *= len(p.Values)
	}
	fmt.Printf("  Итого комбинаций: %d (%s)\n\n", total, multStr(params))

	cfg := askOR(reader, params)

	fmt.Println("─── Генерация ────────────────────────────────────────")
	f := excelize.NewFile()
	defer f.Close()
	st := mkStyles(f)

	fmt.Print("  Строю ТПР... ")
	buildTPR(f, params, cfg, st)
	fmt.Println("готово")

	f.DeleteSheet("Sheet1")
	if err := f.SaveAs(outputFile); err != nil {
		fmt.Printf("\n  [!] Ошибка сохранения: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════")
	fmt.Printf("  Файл: %s\n", outputFile)
	fmt.Printf("  TPR:  %d тест-кейсов (%s)\n", total, multStr(params))
	fmt.Println("══════════════════════════════════════════════════════")
	fmt.Println()
}

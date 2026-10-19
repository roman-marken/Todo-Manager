package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------- Модели ----------

type Priority int

const (
	Low Priority = iota
	Medium
	High
)

func (p Priority) String() string {
	switch p {
	case High:
		return "ВЫСОКИЙ"
	case Medium:
		return "СРЕДНИЙ"
	default:
		return "НИЗКИЙ"
	}
}

type Task struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Done      bool       `json:"done"`
	Priority  Priority   `json:"priority"`
	CreatedAt time.Time  `json:"created_at"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

type Store struct {
	Tasks  []Task `json:"tasks"`
	NextID int    `json:"next_id"`
	path   string
}

// ---------- Работа с хранилищем ----------

func NewStore(path string) (*Store, error) {
	s := &Store{path: path, NextID: 1}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	if s.NextID < 1 {
		s.NextID = 1
		for _, t := range s.Tasks {
			if t.ID >= s.NextID {
				s.NextID = t.ID + 1
			}
		}
	}
	return s, nil
}

func (s *Store) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) Add(title string, p Priority) Task {
	t := Task{
		ID:        s.NextID,
		Title:     title,
		Priority:  p,
		CreatedAt: time.Now(),
	}
	s.NextID++
	s.Tasks = append(s.Tasks, t)
	return t
}

func (s *Store) Get(id int) (*Task, int) {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			return &s.Tasks[i], i
		}
	}
	return nil, -1
}

func (s *Store) Delete(id int) bool {
	_, idx := s.Get(id)
	if idx < 0 {
		return false
	}
	s.Tasks = append(s.Tasks[:idx], s.Tasks[idx+1:]...)
	return true
}

// ---------- Цветной вывод ----------

const (
	cReset  = "\033[0m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
	cGray   = "\033[90m"
	cBold   = "\033[1m"
)

func coloredPriority(p Priority) string {
	switch p {
	case High:
		return cRed + p.String() + cReset
	case Medium:
		return cYellow + p.String() + cReset
	default:
		return cCyan + p.String() + cReset
	}
}

func printTask(t Task) {
	mark := cGray + "[ ]" + cReset
	title := t.Title
	if t.Done {
		mark = cGreen + "[✓]" + cReset
		title = cGray + strike(t.Title) + cReset
	}
	fmt.Printf("  %s #%-3d %s  %s\n",
		mark, t.ID, coloredPriority(t.Priority), title)
}

func strike(s string) string {
	// Простое «зачёркивание» — оборачиваем в символы, если терминал не поддерживает,
	// просто приглушим цветом.
	return s
}

// ---------- Фильтры и сортировки ----------

type Filter int

const (
	All Filter = iota
	OnlyActive
	OnlyDone
)

func (s *Store) List(f Filter, sortByPriority bool) []Task {
	var res []Task
	for _, t := range s.Tasks {
		switch f {
		case OnlyActive:
			if t.Done {
				continue
			}
		case OnlyDone:
			if !t.Done {
				continue
			}
		}
		res = append(res, t)
	}
	sort.SliceStable(res, func(i, j int) bool {
		if sortByPriority && res[i].Priority != res[j].Priority {
			return res[i].Priority > res[j].Priority
		}
		return res[i].ID < res[j].ID
	})
	return res
}

// ---------- Парсер ввода ----------

func parsePriority(s string) Priority {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "h", "high", "в", "высокий":
		return High
	case "m", "medium", "с", "средний":
		return Medium
	default:
		return Low
	}
}

func parseID(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// ---------- CLI ----------

type App struct {
	store  *Store
	reader *bufio.Reader
}

func NewApp(store *Store) *App {
	return &App{store: store, reader: bufio.NewReader(os.Stdin)}
}

func (a *App) readLine(prompt string) string {
	fmt.Print(prompt)
	line, _ := a.reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func (a *App) help() {
	fmt.Println(cBold + "\nКоманды:" + cReset)
	fmt.Println("  add <текст>         — добавить задачу")
	fmt.Println("  add! <текст>        — добавить с высоким приоритетом")
	fmt.Println("  list                — список (все)")
	fmt.Println("  list a              — только активные")
	fmt.Println("  list d              — только выполненные")
	fmt.Println("  list p              — сортировка по приоритету")
	fmt.Println("  done <id>           — отметить выполненной")
	fmt.Println("  undone <id>         — снять отметку")
	fmt.Println("  del <id>            — удалить")
	fmt.Println("  clear               — удалить все выполненные")
	fmt.Println("  stats               — статистика")
	fmt.Println("  help                — эта справка")
	fmt.Println("  exit                — выход")
}

func (a *App) printList(f Filter, byPriority bool) {
	tasks := a.store.List(f, byPriority)
	if len(tasks) == 0 {
		fmt.Println(cGray + "  (пусто)" + cReset)
		return
	}
	for _, t := range tasks {
		printTask(t)
	}
}

func (a *App) stats() {
	total, done := len(a.store.Tasks), 0
	for _, t := range a.store.Tasks {
		if t.Done {
			done++
		}
	}
	pending := total - done
	pct := 0.0
	if total > 0 {
		pct = float64(done) / float64(total) * 100
	}
	fmt.Printf("\nВсего: %d  |  Выполнено: %s%d%s  |  Активных: %s%d%s  |  Прогресс: %.1f%%\n",
		total, cGreen, done, cReset, cYellow, pending, cReset, pct)
}

func (a *App) Run() {
	fmt.Println(cBold + "=== Todo Manager ===" + cReset)
	a.help()

	for {
		line := a.readLine("\n> ")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToLower(parts[0])
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch cmd {
		case "add", "add!":
			if arg == "" {
				fmt.Println(cRed + "Укажи текст задачи" + cReset)
				continue
			}
			p := Low
			if cmd == "add!" {
				p = High
			}
			t := a.store.Add(arg, p)
			a.saveAndReport("Добавлено", t)
			printTask(t)

		case "list":
			f := All
			byP := false
			for _, r := range strings.Fields(arg) {
				switch strings.ToLower(r) {
				case "a", "active":
					f = OnlyActive
				case "d", "done":
					f = OnlyDone
				case "p", "priority":
					byP = true
				}
			}
			a.printList(f, byP)

		case "done", "undone", "del":
			id, err := parseID(arg)
			if err != nil {
				fmt.Println(cRed + "Неверный ID" + cReset)
				continue
			}
			t, idx := a.store.Get(id)
			if idx < 0 {
				fmt.Println(cRed + "Задача не найдена" + cReset)
				continue
			}
			switch cmd {
			case "done":
				if !t.Done {
					now := time.Now()
					t.Done = true
					t.DoneAt = &now
				}
				fmt.Println(cGreen + "Отмечено выполненной" + cReset)
			case "undone":
				t.Done = false
				t.DoneAt = nil
				fmt.Println(cYellow + "Отметка снята" + cReset)
			case "del":
				if a.store.Delete(id) {
					fmt.Println(cYellow + "Удалено" + cReset)
				}
			}
			a.saveAndReport("", Task{})

		case "clear":
			var kept []Task
			removed := 0
			for _, t := range a.store.Tasks {
				if t.Done {
					removed++
					continue
				}
				kept = append(kept, t)
			}
			a.store.Tasks = kept
			a.saveAndReport("", Task{})
			fmt.Printf("Удалено выполненных: %d\n", removed)

		case "stats":
			a.stats()

		case "help":
			a.help()

		case "exit", "quit", "q":
			fmt.Println("Пока!")
			return

		default:
			fmt.Println(cRed + "Неизвестная команда. help для справки." + cReset)
		}
	}
}

func (a *App) saveAndReport(prefix string, _ Task) {
	if err := a.store.Save(); err != nil {
		fmt.Println(cRed+"Ошибка сохранения:"+cReset, err)
		return
	}
	if prefix != "" {
		fmt.Println(cGreen + prefix + cReset)
	}
}

// ---------- Точка входа ----------

func main() {
	store, err := NewStore("tasks.json")
	if err != nil {
		fmt.Println("Ошибка инициализации:", err)
		os.Exit(1)
	}
	NewApp(store).Run()
}

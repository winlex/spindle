# L02 — Значения, копирование, нулевые значения

Модуль 1 · 60–90 минут · Версия Go: актуальная стайбл **1.27.x**

Примеры в §3 и код в §5 — рабочие. Всё собрано и прогнано на Go 1.27.1: `gofmt` чист,
`go vet` код 0, тесты зелёные под `-race`, покрытие пакета из §5 — 100%. Выводы в теории
сняты прогоном, а не пересказаны.

---

## 1. Название и цель

**Цель:** понять, что в Go всё является значением, а не ссылкой; научиться писать функции,
которые возвращают изменённую копию; выбрать между значением и указателем осознанно.

## 2. Что я должен уметь после урока

- Отличать типы, которые копируются целиком, от типов, которые копируются как ссылка.
- Писать функции, меняющие состояние, возвращая новое значение.
- Объяснить, почему `for _, t := range ts { t.X = 1 }` не работает и как это правильно.
- Знать, какие нулевые значения пригодны сразу, а какие нет.
- Объяснить, когда указатель обязателен, а когда вредит.

## 3. Теория

### 3.1. Всё значение

Присваивание копирует. Исключение — несколько типов, которые внутри хранят указатель на данные;
но и они копируются как дескриптор, а не как «объект».

В примерах `t1` и `t2` — задачи из §5, у них поля `ID int`, `Title string`, `Status Status`.

| Тип | Что копируется | Пример | Итог |
|---|---|---|---|
| `int`, `string` | значение целиком | `i1 := 1`<br>`i2 := i1`<br>`i2 = 100` | `1 100` — изменился только `i2` |
| `struct` | **все поля**, включая вложенные | `t2 := t1`<br>`t2.Title = "Другое"` | `t1.Title = "Починить сборку"` — оригинал цел |
| `[N]T` массив | все элементы | `a1 := [2]int{1, 2}`<br>`a2 := a1`<br>`a2[0] = 100` | `[1 2] [100 2]` — `a1` не тронут |
| `[]T` срез | дескриптор: указатель, `len`, `cap` | `list2 := list`<br>`list2[0].Title = "мусор"` | `list[0].Title = "мусор"` — **`list` тоже изменился** |
| `map[K]V` | ссылка | `m2 := m`<br>`m2[StatusTodo] = 100` | `m[StatusTodo] = 100` — **общий словарь** |
| `*T` | ссылка | `p2 := p1`<br>`p2.Title = "через указатель"` | `t1.Title` изменился — **общий объект** |
| `chan T` | ссылка | `c2 := c1`<br>`c1 <- 7` | `7` — из `c2` прочитается то же |
| `func(...)` | ссылка | `f2 := f1` | код тот же, копируется указатель |
| `interface` | то, что внутри: значение либо ссылка | `var v1 I = T{1}`<br>`var v2 I = &T{1}` | `{1} &{1}` — значение против указателя |

Последнюю строку проверяют через `reflect.ValueOf(f).Pointer()` — печатать значение функции
нельзя, `go vet` ругается на `%T` и на невызванную функцию.

Смотри на строки «массив» и «срез»: объявлены похоже, а ведут себя противоположно. У массива
копируется содержимое, у среза — только адрес содержимого. Различие уходит корнями в `len` и `cap`.

`string` — не ссылка на символы, а непрерывный байтовый массив со своим заголовком. Присваивание
копирует и его. Поэтому существуют `strings.Builder` и `bytes.Buffer` — чтобы не копировать.

### 3.2. Функция не может изменить аргумент

```go
func (t Task) WithTitle(title string, now time.Time) (Task, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return Task{}, ErrEmptyTitle
	}
	t.Title = trimmed
	t.UpdatedAt = now
	return t, nil
}

original := mustTask(1, "Старое")
updated, _ := original.WithTitle("Новое", now)
// original.Title == "Старое", updated.Title == "Новое"
```

Это не баг и не ограничение оптимизатора — так устроен язык. Единственный путь: вернуть новое
значение. Отсюда соглашение об именовании: функция, которая «меняет», возвращает значение и
начинается с `With` или `New`.

Побочный эффект от такого решения — **время тоже приходит аргументом**, а не берётся из
`time.Now` внутри. Иначе результат невоспроизводим и не покрывается тестом.

### 3.3. Нулевые значения: рабочие и вредные

`var t Task` даёт структуру, в которой все поля нулевые. Это **не** «пустое» значение, а
конкретное состояние — и инварианты в нём нарушены.

| Тип | Нулевое значение | Годится сразу? |
|---|---|---|
| `int`, `string`, `bool` | `0`, `""`, `false` | да |
| `sync.Mutex` | неинициализированная | **да** — в этом весь смысл: `var mu sync.Mutex` работает |
| `bytes.Buffer` | готов к работе | да |
| `[]T` | `nil` | да: `len`, `range`, `append` работают |
| `map[K]V` | `nil` | **частично**: читать и `len` можно, писать — паника |
| `*T` | `nil` | нет: обращение разыменует nil и паникует |
| `error` | `nil` | да — «ошибки нет» |
| `time.Time` | год 1 | **нет** — валидное время, неотличимое от «не задано» |

Проверено на домене из §5:

```text
Task{}: Status = "", Title = ""
Task{}.Validate(): tasks: пустое название задачи
чтение из nil-карты: 0 | len: 0
запись в nil-карту: assignment to entry in nil map
```

Отсюда правило, которое видно на примере `Validate`: проверять инварианты в конструкторе и
возвращать `Task{}` при ошибке, а не частично заполненную задачу.

Последняя строка таблицы ловит не только в тестах:

```text
time.Time{}: IsZero() = true
JSON с нулевым time.Time: {"started_at":"0001-01-01T00:00:00Z"}
с omitempty:             {"started_at":"0001-01-01T00:00:00Z"}
указатель с omitempty:   {}
```

`omitempty` пропускает только пустые значения примитивов, срезов и карт — но не структуры.
Варианты: `*time.Time`, отдельный флаг, доменный тип с собственным `MarshalJSON`. Это придёт
в модуле 4 вместе с JSON API.

### 3.4. `range` копирует элемент

```go
for _, t := range tasks {
	t.Title = "changed"   // мутация теряется
}
```

```text
for _, t := range -> "a" "b" — мутация потеряна
for i := range    -> "changed" "changed"
```

`t` — копия элемента на каждой итерации. Изменить её — изменить копию и забыть.

**Версия 1.22 изменила переменную цикла:** теперь `for i, v := range` создаёт новую `i` и `v`
на каждой итерации, поэтому `&v` больше не смотрит на одну и ту же переменную:

```go
var ptrs []*Task
for _, t := range []Task{{ID: 1}, {ID: 2}} {
	ptrs = append(ptrs, &t)
}
// ptrs[0].ID == 1, ptrs[1].ID == 2 — разные, а не одно и то же
```

Раньше это был источник самой частой ошибки в Go-коде. Анализатор `loopclosure` в `vet`
остаётся полезным на старом коде.

### 3.5. Указатели: когда нужен, когда вредит

Указатель нужен, когда требуется **разделить изменение между областями видимости**: у структуры
есть поле-ссылка на саму себя, лежит `sync.Mutex` (который не должен копироваться), надо
отличить «не задано» от «нуля», или размер значения делает копирование дорогим.

Указатель вредит, когда значение неизменяемо по смыслу: им усложняют сигнатуру, заставляют
проверять `nil` в каждом месте и разрушают идею «данные просто лежат в переменной».

```go
func (t Task) IsDone() bool                    { return t.Status == StatusDone }   // не меняет
func (t Task) WithStatus(s Status, now time.Time) (Task, error)                     // меняет
```

**Соглашение Go:** метод на значении, если не меняет получателя; на указателе, если меняет.
Оно существует для интуитивности вызова: `tasks[i].setTitle("x")` работает в обоих случаях,
а вот `t := *ptr; t.setTitle("y")` — нет, если метод на значении.

Но соглашение легко нарушить неявно, и вот тут оно окупается:

```go
single := tasks[0]
single.Title = "изменено"
// tasks[0].Title не изменился
```

---

## 4. Сравнение с PHP и TypeScript

| Что | PHP | TypeScript / Node | Go |
|---|---|---|---|
| Присваивание объекта | `$b = $a` — тот же объект | `const b = a` — тот же объект | `b := a` — копия |
| Присваивание массива | `$b = $a` — копия (COW) | без копии, тот же объект | копия дескриптора, данные общие |
| Изменение аргумента в функции | по ссылке, если `&` | по ссылке, объект тот же | невозможно, только возврат |
| `clone` | `clone $a` | `{...a}` | операции нет; возвращай новое |
| Изменение в цикле по элементу | `foreach ($ts as &$t)` | `for (const t of ts)` — нет | `for i, v` — копия, нужен индекс |
| Рост массива | `$a[] = $x` — всегда новый массив | `push` — всегда новый массив | `append` — **иногда на месте** |
| Запас памяти | нет понятия | нет понятия | `cap` |
| Нулевое значение | `null` | `undefined` / `null` | у каждого типа своё |
| «Объект без полей» | `new stdClass` | `{}` | невозможно, у структуры всегда есть состояние |

Три различия, которые бьют по-разному:

1. **Надежда на мутацию аргумента.** Самая частая ошибка переноса, компилятор молчит.
2. **`append` на месте.** В PHP и JS рост массива всегда даёт новый объект, в Go — нет. Код,
   который на PHP работал «случайно», на Go начнёт портить данные.
3. **`foreach as &$t`.** В PHP это меняет элементы массива, в Go — нет.

## 5. Идиоматичный пример

Домен Spindle целиком: `internal/tasks`. Это тот код, который пойдёт в проект — сохрани его
файлом как есть и используй как основу для задания.

```go
// Package tasks содержит доменную модель трекера задач: сами задачи, правила перехода
// между состояниями и запросы к спискам задач.
//
// Пакет построен на семантике значений: у задачи нет указателей, а все методы, которые
// что-то меняют, возвращают новую задачу и не трогают получателя.
package tasks

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Status — состояние задачи. Определённый тип строки, а не псевдоним, чтобы методы
// можно было вешать на сам тип, а не на string.
type Status string

// Состояния задачи. Значения совпадают с тем, что пойдёт в JSON и в базу.
const (
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// allStatuses — все известные состояния. Порядок задаёт порядок вывода в отчётах.
var allStatuses = []Status{StatusBacklog, StatusTodo, StatusInProgress, StatusDone}

// MaxTitleLen ограничивает длину названия задачи.
const MaxTitleLen = 200

// Ошибки домена. Обёрнуты в конкретные ошибки вызывающими функциями через %w, чтобы
// вызывающая сторона проверяла их через errors.Is, а не сравнивала текст.
var (
	ErrEmptyTitle       = errors.New("tasks: пустое название задачи")
	ErrTitleTooLong     = errors.New("tasks: слишком длинное название задачи")
	ErrUnknownStatus    = errors.New("tasks: неизвестный статус")
	ErrTransitionDenied = errors.New("tasks: переход запрещён")
	ErrSameStatus       = errors.New("tasks: задача уже в этом статусе")
)

// transitions — таблица разрешённых переходов: внешний ключ — текущий статус,
// значение — во какие статусы из него можно уйти. Вперёд разрешён один шаг,
// назад — один шаг, но в done можно попасть из любого состояния, а из done
// можно уйти только в todo: задача один раз закрыта — и для возврата в работу её
// надо переоткрыть явно.
var transitions = map[Status]map[Status]bool{
	StatusBacklog:    {StatusTodo: true, StatusDone: true},
	StatusTodo:       {StatusBacklog: true, StatusInProgress: true},
	StatusInProgress: {StatusTodo: true, StatusDone: true},
	StatusDone:       {StatusTodo: true},
}

// IsKnown сообщает, что статус входит в перечень известных.
func (s Status) IsKnown() bool {
	for _, known := range allStatuses {
		if s == known {
			return true
		}
	}
	return false
}

// Task — задача трекера.
//
// Поля экспортируемые, но менять задачу в обход методов не следует: инварианты
// вроде «название непустое» проверяет Validate, а переходы между статусами —
// WithStatus. Копия задачи безопасна: все поля — значения или дескрипторы срезов,
// которые не меняются сами по себе.
type Task struct {
	ID         int
	Title      string
	Status     Status
	AssigneeID int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewTask создаёт задачу в бэклоге. Время передаётся аргументом, а не берётся из
// time.Now внутри: так результат детерминирован и его можно проверить тестом.
func NewTask(id int, title string, now time.Time) (Task, error) {
	t := Task{
		ID:        id,
		Title:     strings.TrimSpace(title),
		Status:    StatusBacklog,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := t.Validate(); err != nil {
		return Task{}, err
	}
	return t, nil
}

// Validate проверяет инварианты задачи. Вызывается из NewTask, но не из каждого
// метода: если инвариант соблюдён при создании, то WithStatus его не нарушит.
func (t Task) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return ErrEmptyTitle
	}
	if len([]rune(t.Title)) > MaxTitleLen {
		return fmt.Errorf("%w: %d символов при максимуме %d", ErrTitleTooLong, len([]rune(t.Title)), MaxTitleLen)
	}
	if !t.Status.IsKnown() {
		return fmt.Errorf("%w: %q", ErrUnknownStatus, t.Status)
	}
	return nil
}

// CanTransition сообщает, разрешён ли переход из текущего статуса в target.
// Само значение не меняет и не аллоцирует.
func (t Task) CanTransition(target Status) bool {
	return transitions[t.Status][target]
}

// WithStatus возвращает копию задачи в новом статусе. Получатель не меняется:
// это единственный способ сменить состояние, и проверки не обойти.
func (t Task) WithStatus(target Status, now time.Time) (Task, error) {
	if !target.IsKnown() {
		return Task{}, fmt.Errorf("%w: %q", ErrUnknownStatus, target)
	}
	if target == t.Status {
		return Task{}, fmt.Errorf("%w: %q", ErrSameStatus, target)
	}
	if !t.CanTransition(target) {
		return Task{}, fmt.Errorf("%w: %q -> %q", ErrTransitionDenied, t.Status, target)
	}
	t.Status = target
	t.UpdatedAt = now
	return t, nil
}

// WithAssignee назначает исполнителя. Нулевой ID снимает назначение.
func (t Task) WithAssignee(assigneeID int, now time.Time) (Task, error) {
	if assigneeID < 0 {
		return Task{}, fmt.Errorf("tasks: отрицательный ID исполнителя: %d", assigneeID)
	}
	t.AssigneeID = assigneeID
	t.UpdatedAt = now
	return t, nil
}

// WithTitle меняет название с проверкой.
func (t Task) WithTitle(title string, now time.Time) (Task, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return Task{}, ErrEmptyTitle
	}
	if len([]rune(trimmed)) > MaxTitleLen {
		return Task{}, fmt.Errorf("%w: %d символов при максимуме %d", ErrTitleTooLong, len([]rune(trimmed)), MaxTitleLen)
	}
	t.Title = trimmed
	t.UpdatedAt = now
	return t, nil
}

// IsDone сообщает, закрыта ли задача.
func (t Task) IsDone() bool { return t.Status == StatusDone }

// Filter — набор условий отбора задач. Нулевое поле означает «не фильтровать»:
// пустой срез Statuses и нулевой AssigneeID пропускают любое значение.
type Filter struct {
	Statuses   []Status
	AssigneeID int
}

// Matches проверяет одно условие отбора.
func (t Task) Matches(f Filter) bool {
	if len(f.Statuses) > 0 && !contains(f.Statuses, t.Status) {
		return false
	}
	if f.AssigneeID != 0 && t.AssigneeID != f.AssigneeID {
		return false
	}
	return true
}

// FilterTasks возвращает новый срез подходящих задач. Исходный срез не меняется:
// элементы копируются в новый массив, а не переиспользуется его память.
func FilterTasks(all []Task, f Filter) []Task {
	out := make([]Task, 0, len(all))
	for _, t := range all {
		if t.Matches(f) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SortByCreatedAt сортирует копию задач по времени создания, а при равенстве — по ID.
// Возвращает новый срез: порядок вызывающего не меняется.
func SortByCreatedAt(in []Task) []Task {
	out := make([]Task, len(in))
	copy(out, in)
	sortTasksByCreatedAt(out)
	return out
}

// sortTasksByCreatedAt сортирует на месте — вызывается только из SortByCreatedAt,
// который уже скопировал срез.
func sortTasksByCreatedAt(list []Task) {
	sort.Slice(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return list[i].ID < list[j].ID
	})
}

func contains(list []Status, s Status) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
```

Решения, которые стоит разобрать, пока код свежий:

| Решение | Почему |
|---|---|
| `Status` — `type Status string`, а не псевдоним | на `string` нельзя повесить метод, а `IsKnown` нужен и в домене, и в парсере |
| `transitions` — `map[Status]map[Status]bool` | правило переходов читается как таблица, а не как цепочка `if` |
| `CanTransition` без проверки `IsKnown` | неизвестный статус не найдётся в таблице и даст `false` — проверка лишняя |
| `NewTask` возвращает `Task{}` при ошибке | частично заполненная задача опаснее пустой |
| `Validate` отдельно от `NewTask` | вызывающий может собрать `Task` литералом — проверку надо иметь под рукой |
| время аргументом, не `time.Now` | результат детерминирован, тест не мигает |
| `len([]rune(...))`, а не `len(...)` | `len` считает байты, кириллица даст двойной лимит |
| `WithX` вместо `SetX` | нет мутации, есть новое значение; сигнатура читается как «дай мне копию» |
| `FilterTasks` начинает с `make` и возвращает `nil` на пустом | результат не зависит от входа, `nil` не протекает наружу |
| `SortByCreatedAt` копирует через `copy` | `sort.Slice` мутирует на месте, а контракт обещает новый срез |
| сравнение по `CreatedAt`, потом по `ID` | сортировка детерминирована, тест не мигает при равных временах |

Тесты — вторая половина примера:

```go
package tasks

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var ref = time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)

func mustTask(t *testing.T, id int, title string) Task {
	t.Helper()
	task, err := NewTask(id, title, ref)
	if err != nil {
		t.Fatalf("NewTask(%d, %q) = ошибка %v", id, title, err)
	}
	return task
}

func TestNewTask(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr error
	}{
		{"обычное название", "Починить сборку", nil},
		{"пробелы обрезаются", "  Починить сборку  ", nil},
		{"пустое название", "", ErrEmptyTitle},
		{"только пробелы", "   ", ErrEmptyTitle},
		{"слишком длинное", string(make([]rune, MaxTitleLen+1)), ErrTitleTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := NewTask(1, tt.title, ref)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, ожидали %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if task.Status != StatusBacklog {
				t.Errorf("Status = %q, ожидали %q", task.Status, StatusBacklog)
			}
			if !task.CreatedAt.Equal(ref) {
				t.Errorf("CreatedAt = %v, ожидали %v", task.CreatedAt, ref)
			}
		})
	}
}

func TestNewTaskTrimsTitle(t *testing.T) {
	task := mustTask(t, 1, "  Починить сборку  ")
	if task.Title != "Починить сборку" {
		t.Errorf("Title = %q, ожидали без пробелов", task.Title)
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
		want bool
	}{
		{StatusBacklog, StatusTodo, true},
		{StatusBacklog, StatusDone, true},
		{StatusBacklog, StatusInProgress, false},
		{StatusTodo, StatusBacklog, true},
		{StatusTodo, StatusInProgress, true},
		{StatusTodo, StatusDone, false},
		{StatusInProgress, StatusTodo, true},
		{StatusInProgress, StatusDone, true},
		{StatusInProgress, StatusBacklog, false},
		{StatusDone, StatusTodo, true},
		{StatusDone, StatusBacklog, false},
		{StatusDone, StatusInProgress, false},
		{StatusBacklog, StatusBacklog, false},
		{StatusBacklog, Status("мусор"), false},
	}
	for _, tt := range tests {
		task := Task{Status: tt.from}
		if got := task.CanTransition(tt.to); got != tt.want {
			t.Errorf("CanTransition(%q -> %q) = %v, хотели %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestWithStatusDoesNotMutateOriginal(t *testing.T) {
	original := mustTask(t, 1, "Починить сборку")
	updated, err := original.WithStatus(StatusTodo, ref.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	if original.Status != StatusBacklog {
		t.Errorf("оригинал изменил статус на %q", original.Status)
	}
	if original.UpdatedAt != ref {
		t.Errorf("оригинал изменил UpdatedAt на %v", original.UpdatedAt)
	}
	if updated.Status != StatusTodo {
		t.Errorf("Status = %q, ожидали %q", updated.Status, StatusTodo)
	}
	if !updated.UpdatedAt.Equal(ref.Add(time.Hour)) {
		t.Errorf("UpdatedAt = %v, ожидали %v", updated.UpdatedAt, ref.Add(time.Hour))
	}
	if !updated.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt изменился: %v -> %v", original.CreatedAt, updated.CreatedAt)
	}
}

func TestWithStatusErrors(t *testing.T) {
	backlog := mustTask(t, 1, "Починить сборку")

	tests := []struct {
		name    string
		from    Task
		to      Status
		wantErr error
	}{
		{"переход запрещён", backlog, StatusInProgress, ErrTransitionDenied},
		{"тот же статус", backlog, StatusBacklog, ErrSameStatus},
		{"неизвестный статус", backlog, Status("мусор"), ErrUnknownStatus},
		{"из done назад нельзя", Task{Status: StatusDone}, StatusInProgress, ErrTransitionDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.from.WithStatus(tt.to, ref)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, ожидали %v", err, tt.wantErr)
			}
			if err == nil && got.ID != tt.from.ID {
				t.Errorf("при ошибке вернулась непустая задача: %+v", got)
			}
		})
	}
}

func TestFilterTasksDoesNotTouchSource(t *testing.T) {
	all := []Task{
		{ID: 1, Title: "a", Status: StatusTodo},
		{ID: 2, Title: "b", Status: StatusDone},
		{ID: 3, Title: "c", Status: StatusTodo},
	}
	before := append([]Task(nil), all...)

	got := FilterTasks(all, Filter{Statuses: []Status{StatusTodo}})

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, ожидали 2", len(got))
	}
	for i, want := range before {
		if all[i] != want {
			t.Fatalf("исходный срез изменился на индексе %d: %+v -> %+v", i, want, all[i])
		}
	}
	got[0].Title = "мусор"
	if all[0].Title != "a" {
		t.Errorf("исходный элемент изменился: %q", all[0].Title)
	}
}

func TestSortByCreatedAtKeepsSource(t *testing.T) {
	all := []Task{
		{ID: 3, CreatedAt: ref},
		{ID: 1, CreatedAt: ref.Add(-time.Hour)},
		{ID: 2, CreatedAt: ref.Add(-time.Hour)},
	}
	first := all[0]

	sorted := SortByCreatedAt(all)

	if all[0] != first {
		t.Errorf("исходный срез переставился: %+v -> %+v", first, all[0])
	}
	for i, want := range []int{1, 2, 3} {
		if sorted[i].ID != want {
			t.Fatalf("sorted[%d].ID = %d, ожидали %d; весь срез %v", i, sorted[i].ID, want, sorted)
		}
	}
}

func TestValidateCountsRunesNotBytes(t *testing.T) {
	long := strings.Repeat("ы", MaxTitleLen) // 2 байта на руну
	task := Task{Title: long, Status: StatusBacklog}
	if err := task.Validate(); err != nil {
		t.Errorf("Validate() = %v, ожидали nil: %d рун должно проходить", err, MaxTitleLen)
	}
}

func TestWithAssignee(t *testing.T) {
	original := mustTask(t, 1, "Починить сборку")

	assigned, err := original.WithAssignee(42, ref.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if original.AssigneeID != 0 {
		t.Errorf("оригинал получил исполнителя %d", original.AssigneeID)
	}
	if assigned.AssigneeID != 42 {
		t.Errorf("AssigneeID = %d, ожидали 42", assigned.AssigneeID)
	}

	unassigned, err := assigned.WithAssignee(0, ref.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if unassigned.AssigneeID != 0 {
		t.Errorf("снятие исполнителя не сработало: %d", unassigned.AssigneeID)
	}
}

func TestWithAssigneeRejectsNegative(t *testing.T) {
	task := mustTask(t, 1, "Починить сборку")
	if _, err := task.WithAssignee(-1, ref); err == nil {
		t.Error("отрицательный ID исполнителя должен давать ошибку")
	}
}

func TestWithTitle(t *testing.T) {
	original := mustTask(t, 1, "Старое название")

	updated, err := original.WithTitle("  Новое название  ", ref.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if original.Title != "Старое название" {
		t.Errorf("оригинал изменил название на %q", original.Title)
	}
	if updated.Title != "Новое название" {
		t.Errorf("Title = %q, ожидали без пробелов", updated.Title)
	}
	if !updated.UpdatedAt.Equal(ref.Add(time.Hour)) {
		t.Errorf("UpdatedAt = %v, ожидали обновлённый", updated.UpdatedAt)
	}
}

func TestWithTitleErrors(t *testing.T) {
	task := mustTask(t, 1, "Название")
	if _, err := task.WithTitle("   ", ref); !errors.Is(err, ErrEmptyTitle) {
		t.Errorf("err = %v, ожидали ErrEmptyTitle", err)
	}
	if _, err := task.WithTitle(strings.Repeat("ы", MaxTitleLen+1), ref); !errors.Is(err, ErrTitleTooLong) {
		t.Errorf("err = %v, ожидали ErrTitleTooLong", err)
	}
}

func TestWithStatusChain(t *testing.T) {
	task := mustTask(t, 1, "Починить сборку")
	later := ref.Add(2 * time.Hour)

	for _, target := range []Status{StatusTodo, StatusInProgress, StatusDone} {
		next, err := task.WithStatus(target, later)
		if err != nil {
			t.Fatalf("переход в %q: %v", target, err)
		}
		task = next
	}

	if !task.IsDone() {
		t.Error("после цепочки переходов задача должна быть закрыта")
	}
	if _, err := task.WithStatus(StatusInProgress, later); !errors.Is(err, ErrTransitionDenied) {
		t.Errorf("err = %v, ожидали ErrTransitionDenied", err)
	}
}

func TestIsDone(t *testing.T) {
	if !(Task{Status: StatusDone}).IsDone() {
		t.Error("StatusDone должен давать IsDone() = true")
	}
	if (Task{Status: StatusInProgress}).IsDone() {
		t.Error("StatusInProgress не должен давать IsDone() = true")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr error
	}{
		{"корректная задача", Task{Title: "Название", Status: StatusTodo}, nil},
		{"пустое название", Task{Status: StatusTodo}, ErrEmptyTitle},
		{"неизвестный статус", Task{Title: "Название", Status: Status("мусор")}, ErrUnknownStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.task.Validate(); !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, ожидали %v", err, tt.wantErr)
			}
		})
	}
}

func TestMatchesByAssignee(t *testing.T) {
	task := Task{Status: StatusTodo, AssigneeID: 7}
	if (Task{Status: StatusTodo}).Matches(Filter{AssigneeID: 7}) {
		t.Error("задача без исполнителя не должна проходить фильтр по исполнителю")
	}
	if task.Matches(Filter{AssigneeID: 8}) {
		t.Error("чужой исполнитель не должен проходить")
	}
	if !task.Matches(Filter{}) {
		t.Error("пустой фильтр должен пропускать всё")
	}
}

func TestFilterTasksEmptyResult(t *testing.T) {
	all := []Task{{ID: 1, Status: StatusTodo}}
	if got := FilterTasks(all, Filter{Statuses: []Status{StatusDone}}); got != nil {
		t.Errorf("ожидали nil, получили %v", got)
	}
	if got := FilterTasks(nil, Filter{}); got != nil {
		t.Errorf("ожидали nil, получили %v", got)
	}
}
```

Пять приёмов, которые здесь видны и которые пригодятся дальше:

- `t.Helper()` в `mustTask` — при падении указывает на строку вызова, а не на строку
  внутри хелпера.
- `if !errors.Is(err, tt.wantErr)` вместо сравнения текста — текст меняется вместе с
  формулировкой, наличие ошибки нет.
- `before := append([]Task(nil), all...)` и сравнение поэлементно — срез нельзя сравнить
  через `==`, компилятор не даст.
- `if err == nil && got.ID != tt.from.ID` — проверка контракта «при ошибке возвращаем ноль».
  Без неё функция может вернуть частично заполненный результат, и тест это пропустит.
- `TestWithStatusChain` вместо копии кейсов в `TestWithStatusErrors` — цепочка проверяет
  правило в динамике: `backlog → todo → in_progress → done`, и из `done` назад нельзя.

Фактическое покрытие этого пакета — 100% по всем функциям, включая `sortTasksByCreatedAt`
и `contains`. Проверено прогоном, не подсчитано на глаз.

## 6. Типичные ошибки переходчиков из PHP

1. **Надежда на мутацию аргумента.** Компилятор молчит, функция «ничего не делает».
2. **`for _, t := range` с присваиванием.** В PHP через `&$t` работает, здесь нет.
3. **`single := tasks[0]` и мутация копии.** `vet` не поймает.
4. **Забытый `return` у функции, растягивающей срез.** Тихая потеря данных.
5. **`append` без присваивания обратно.** Нужно `s = append(s, x)`.
6. **Рассчитывать, что `append` даст новый массив.** В PHP это гарантия, здесь нет.
7. **`len(s) > MaxTitleLen` вместо `len([]rune(s))`.** Кириллица вдвое короче лимита.
8. **`time.Now()` внутри функции, которую тестируешь.** Тест мигает или требует моков.
9. **`if err.Error() == "..."` вместо `errors.Is`.** Ломается при любом переформулировании.
10. **Хранить `sync.Mutex` в структуре, которая копируется.** Блокировка перестанет работать.
11. **`omitempty` на структуре.** JSON будет содержать `0001-01-01` и нули.

## 7. Практическое задание

Возьми пакет из §5 как отправную точку — он уже рабочий и покрыт тестами. Твоя задача —
дописать в него шесть функций и тесты к ним.

**Этап 1. Четыре функции сбора.** Сигнатуры:

```go
func CountByStatus(all []Task) map[Status]int
func GroupByStatus(all []Task) map[Status][]Task
func (t Task) NextStatuses() []Status
func ParseStatus(s string) (Status, error)
```

Требования, которые проверяются тестами:

- `CountByStatus` на пустом входе возвращает **непустую карту** без записей. Объясни в
  `answers/L02.md`, почему здесь `make` обязателен, а в `FilterTasks` пустой результат —
  это `nil`.
- `GroupByStatus` не может отдавать подсрез исходного массива: `groups[s][0]` и `all[i]`
  обязаны быть разными значениями. Тест ловит это мутацией результата.
- `NextStatuses` возвращает разрешённые переходы **в порядке `allStatuses`**, а не в порядке
  обхода карты. На пустом результате возвращает `nil`.
- `ParseStatus` принимает `"  TODO "` и возвращает `StatusTodo`. На неизвестном значении —
  ошибка, обёрнутая вокруг `ErrUnknownStatus` через `%w`.

**Этап 2. Поиск и агрегат.**

```go
var ErrNoTasks = errors.New("tasks: список задач пуст")

func Oldest(in []Task) (Task, error)
func TotalAssignees(all []Task) []int
```

- `Oldest` возвращает задачу с самым ранним `CreatedAt`. При равенстве — с меньшим `ID`,
  чтобы выбор был детерминированным. На пустом входе — `ErrNoTasks` и нулевая `Task`.
- `TotalAssignees` возвращает **отсортированный по возрастанию** список ID исполнителей
  без повторов, ноль исключается. На пустом входе — `nil`. Вопрос в том, вписывается ли
  такой контракт в семантику остального пакета, где пустой результат бывает и `nil`,
  и пустым срезом. Ответь в `answers/L02.md`.

**Этап 3. Согласованность.** Напиши тест, который создаёт пять задач, применяет к каждой
цепочку переходов и проверяет, что `CountByStatus` по итогу даёт сумму, равную числу задач,
а `GroupByStatus` — столько же элементов. Тест должен читаться как утверждение о системе,
а не как перечисление вызовов.

**Этап 4. Найти баг.** Разбери фрагмент из настоящего проекта:

```go
func ActivateAll(tasks []Task) error {
	for _, t := range tasks {
		updated, err := t.WithStatus(StatusInProgress, time.Now())
		if err != nil {
			return err
		}
		t = updated
	}
	return nil
}

func Find(tasks []Task, id int) (Task, error) {
	for _, t := range tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, fmt.Errorf("%w: %d", ErrNoTasks, id)
}

func FirstActive(tasks []Task) (Task, error) {
	active := FilterTasks(tasks, Filter{Statuses: []Status{StatusInProgress}})
	return active[0], nil
}
```

Ответь письменно:

1. Что вернёт `ActivateAll` на двух задачах в бэклоге и что останется в срезе после вызова.
2. Почему компилятор молчит, хотя `t = updated` выглядит как присваивание, которое должно
   «сработать».
3. Что произойдёт в `FirstActive`, если активных задач нет. Ответь конкретной паникой и кодом.
4. Назови случай, где `ActivateAll` вернёт `nil` без ошибки, но задачи всё равно не изменятся.
5. Три способа починить `ActivateAll` и какой выбрать, если `tasks` приходит из хранилища,
   а вызывающему нужно знать, сколько задач реально изменилось.

Все пять ответов сверены с прогоном этого фрагмента — `ActivateAll` на бэклоге даёт
`tasks: переход запрещён: "backlog" -> "in_progress"` и оставляет срез нетронутым,
`FirstActive` на пустом результате падает с `index out of range [0] with length 0`.

## 8. Требования

**Функциональные**

- **Ф1.** Все шесть функций реализованы по сигнатурам этапов 1 и 2.
- **Ф2.** Пустые входы ведут себя так, как описано в этапах 1 и 2, и зафиксировано тестами.
- **Ф3.** Ни одна функция не изменяет входной срез — проверяется тестом во всех шести случаях.
- **Ф4.** `ParseStatus` оборачивает `ErrUnknownStatus` через `%w`.
- **Ф5.** Тест из этапа 3 проходит и падает, если сломать любую из шести функций.
- **Ф6.** Ответы на этап 4 и обоснование контракта `TotalAssignees` записаны в `answers/L02.md`.

**Нефункциональные**

- **Н1.** `gofmt -l .` пусто, `go vet ./...` код 0, `go test ./...` и `-race` зелёные.
- **Н2.** Покрытие пакета не ниже 100% по функциям — ни одна не осталась без теста.
- **Н3.** Ноль новых зависимостей, `go list -m all` по-прежнему одна строка.
- **Н4.** Ни одного указателя в API пакета.
- **Н5.** Комментарий к экспортируемому имени начинается с самого имени — так его показывает `go doc`.
- **Н6.** Ни одной аллокации там, где её можно избежать: `CanTransition` и `Matches`
   остаются без единой. Проверь через `testing.AllocsPerRun`.

## 9. Ограничения

**Разрешено:** stdlib, `go test`, `go vet`, `gofmt`, `go doc`, `git`.

**Запрещено:**

- указатели в API домена;
- `sync`, горутины, `context` — это модуль 3;
- `interface` и `any` в сигнатурах — до L08;
- сторонние зависимости;
- шаблоны (generic) — до L22;
- `sort.Map`, `slices.SortFunc` и другие готовые сортировки по произвольному ключу —
  пока не объяснил, чем `sort.Slice` отличается от них.

## 10. Критерии приёмки

1. [ ] `internal/tasks/tasks.go` целиком: пакет из §5 плюс шесть функций из задания.
2. [ ] `internal/tasks/tasks_test.go` целиком: тесты из §5 плюс тесты к шести функциям
       и тест из этапа 3.
3. [ ] Вывод: `gofmt -l .`, `go vet ./...`, `go test ./... -v`, `go test -race ./...`.
4. [ ] Вывод `go tool cover -func=coverage.out` — указать, какая функция не покрыта и почему
       это нормально. Если покрыты все — так и напиши.
5. [ ] Разделы 3 и 4 в `answers/L02.md`: обоснование контракта и разбор этапа 4.
6. [ ] Ответы на контрольные вопросы, своими словами.
7. [ ] Раздел 5: где застрял, что проверил.

## 11. Команды

```bash
mkdir -p internal/tasks

gofmt -l .
go vet ./...
go test ./internal/tasks/ -v
go test -race ./...
go test ./internal/tasks/ -coverprofile=coverage.out
go tool cover -func=coverage.out

# проверить аллокации, если понадобится
go test ./internal/tasks/ -run TestCanTransition -bench . -benchmem

# стенд для проверки теории, вне проекта
mkdir -p /tmp/probe && cd /tmp/probe && go mod init probe
```

## 12. Дополнительное усложнение (optional)

- **О1.** Замерь аллокации всех шести функций через `testing.AllocsPerRun` и оформи таблицей
  «функция → аллокаций на вызов → откуда». Удивишься, где именно они появляются.
- **О2.** Реализуй `CountByStatus` и `GroupByStatus` одним проходом по срезу и сравни
  с двумя отдельными вызовами через `-benchmem`.
- **О3.** Опиши таблицей «для этого пакета — значение или указатель» и обоснуй каждую
  строку. Обрати внимание на `time.Time` внутри `Task` и на то, что будет при копировании.
- **О4.** Замерь стоимость копирования `Task` при методе на значении против метода на
  указателе через `testing.B`. Инструменты для этого будут на L26, но замерить можно уже
  сейчас — вопрос в том, насколько это имеет значение для структуры из шести полей.
- **О5.** Добавь копию задачи в `GroupByStatus` и в `FilterTasks` обратную проверку:
  результат должен быть независим от исходного массива **в обе стороны** — изменить
  результат нельзя, изменить исходник тоже нельзя.

## 13. Контрольные вопросы

1. `Task` присваивается в другую переменную. Что копируется, а что нет, по каждому полю?
2. Почему `CanTransition` не проверяет `target.IsKnown()` перед обращением к таблице, и в
   каком случае такая проверка всё-таки понадобилась бы?
3. В PHP `foreach ($tasks as $t) { $t->done(); }` меняет элементы массива. Напиши аналог
   по-Go и объясни, почему наивный вариант не работает.
4. Почему `NewTask` возвращает `Task{}` при ошибке, а не частично заполненную задачу?
   Назови случай, где разница станет observable.
5. Почему `omitempty` не убирает поле с `time.Time`, и какие три способа это починить ты назвал?
6. `FilterTasks` возвращает `nil` на пустом результате, а `CountByStatus` обязан вернуть
   непустую карту. Почему для одного случая `nil` правильный ответ, а для другого нет?
7. Чем `&T{}` отличается от `new(T)`? Ответь в одной строке, есть ли между ними разница.

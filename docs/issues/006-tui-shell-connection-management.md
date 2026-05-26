# Issue 006 — TUI shell + connection management screens

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** Issue 001
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md)
**User stories covered:** 1, 3, 4, 5, 6

---

## What to build

TUI skeleton dengan bubbletea: master-password prompt screen, root
navigation, dan screen connection management (list, add, edit, delete,
test). Tidak menyentuh sync execution atau mapping — itu Issue 007/008.

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi rules di `~/.claude/rules/context7.md`. Query wajib:

1. **`github.com/charmbracelet/bubbletea`** — topik: "Model Update View
   pattern, tea.Cmd, key bindings, window size handling, focus management".
2. **`github.com/charmbracelet/bubbles`** — topik: "textinput component,
   list component, table component, spinner".
3. **`github.com/charmbracelet/lipgloss`** — topik: "style composition,
   border, padding, color scheme, terminal width handling".

Ini library yang sering update API — **wajib** cek context7 untuk
versi terbaru sebelum tulis kode.

---

## Step-by-step implementation

### Step 1 — Root app & state

File: `internal/tui/app.go`

```go
type screen int

const (
    screenPasswordPrompt screen = iota
    screenMain
    screenConnList
    screenConnForm
    screenConnTest
)

type model struct {
    current     screen
    history     []screen          // stack untuk back-navigation
    masterKey   []byte
    store       *storage.DB
    width, height int
    err         error

    // child models per screen
    pwdPrompt  passwordPromptModel
    connList   connListModel
    connForm   connFormModel
}

func (m model) Init() tea.Cmd
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m model) View() string
```

Global key bindings:
- `q`, `esc` → back (pop screen) atau quit kalau di root.
- `?` → help overlay.
- `ctrl+c` → quit langsung.

### Step 2 — Master password prompt screen

File: `internal/tui/password_prompt.go`

- Tampilan: textinput dengan `EchoMode = textinput.EchoPassword`.
- Logika sama dengan `config.LoadMasterKey` interactive branch:
  - Kalau salt file belum ada (first run): minta password 2x untuk confirm.
  - Kalau sudah ada: 1x.
  - Validasi: derive key, coba decrypt 1 connection arbitrary (kalau ada)
    untuk verify password benar; kalau gagal → re-prompt dengan pesan
    "wrong master password".
- Setelah sukses: simpan `[]byte` di model, transition ke `screenMain`.
- Master key TIDAK di-log, TIDAK ditulis ke disk.

### Step 3 — Main menu

File: `internal/tui/main_menu.go`

List sederhana dengan opsi:
- Connections
- Tables & Mappings (placeholder, jadi di Issue 007)
- Sync (placeholder, Issue 008)
- History (placeholder, Issue 008)
- Quit

Pakai `list.Model` dari bubbles.

### Step 4 — Connection list screen

File: `internal/tui/conn_list.go`

- `table.Model` dari bubbles, kolom: `NAME | SOURCE | DEST | UPDATED`.
- Load via `storage.ConnectionRepo.List`. Refresh `tea.Cmd` setelah
  perubahan.
- Key bindings:
  - `enter` → detail / edit form.
  - `n` → new connection (form).
  - `t` → test connection (transition ke screenConnTest).
  - `d` → delete (confirm prompt overlay).
  - `r` → reload.

### Step 5 — Connection form (add/edit)

File: `internal/tui/conn_form.go`

Field input (semua pakai `textinput.Model`):
- Name
- Source host, port, user, password, db
- Dest host, port, user, password, db

Navigasi:
- `tab`, `shift+tab` → next/prev field.
- `enter` di field terakhir → submit.
- `esc` → cancel.

Validasi sebelum submit:
- Name required, unique (cek via `GetByName`, kalau add).
- Host required.
- Port: 1-65535.
- Password: tidak boleh empty.

Submit flow:
- Show spinner "Testing source connection..." → `mysql.Open` + ping.
- "Testing dest connection..." → idem.
- Kalau dua-duanya OK: encrypt 2 password, `Insert` (atau `Update` kalau edit).
- Kalau salah satu gagal: tampilkan error inline, prompt "save anyway? (y/N)"
  untuk kasus host belum ready.

Edit mode: pre-fill semua field. Password field bisa kosong → artinya
"jangan ubah password". Kalau diisi → re-encrypt dan update.

### Step 6 — Test connection screen

File: `internal/tui/conn_test.go` (rename dari `conn_test.go` ke
`conn_check.go` untuk hindari konflik dengan Go test convention!).

- Spinner + status text 2 baris:
  - `Source: ✓ OK (mysql 8.0.35)` atau `✗ error msg`.
  - `Dest:   ✓ OK` atau `✗ error msg`.
- Setelah selesai, tunggu key untuk kembali ke list.

### Step 7 — Wire ke `cmd/dbsync/main.go`

Update `runTUI()`:
```go
func runTUI() {
    db, err := storage.Open(defaultDBPath())
    if err != nil { os.Exit(2) }
    defer db.Close()

    p := tea.NewProgram(tui.New(db), tea.WithAltScreen())
    if _, err := p.Run(); err != nil { os.Exit(2) }
}
```

### Step 8 — Style guide (lipgloss)

Definisikan style central di `internal/tui/styles.go`:
- `titleStyle`, `errorStyle`, `successStyle`, `helpStyle`, `borderStyle`.
- Skema warna: monokrom + 1 accent (cyan / green).
- Responsive: terminal < 80 char → simplify table columns.

### Step 9 — Manual QA checklist

(TUI tidak di-unit-test secara mendalam per PRD; manual QA cukup.)

- [ ] Launch `./dbsync` → password prompt muncul.
- [ ] Password salah → re-prompt dengan pesan.
- [ ] Password benar → main menu.
- [ ] Tambah koneksi → form input → test berhasil → row muncul di list.
- [ ] Edit koneksi → ubah host → save → list ter-update.
- [ ] Delete koneksi → confirm → row hilang dari list.
- [ ] Quit dengan `q` → exit clean.

---

## Acceptance criteria

- [ ] Master password prompt: first run minta 2x (confirm), berikutnya 1x. Wrong password → re-prompt.
- [ ] Main menu list + navigasi keyboard berjalan.
- [ ] Connection list table render benar, refresh setelah add/edit/delete.
- [ ] Connection form: tab/shift-tab navigation, validation per field, password masked.
- [ ] Add connection: test koneksi otomatis sebelum simpan, save-anyway opt-in.
- [ ] Edit connection: pre-fill field, password kosong = jangan ubah.
- [ ] Delete connection: confirm prompt, FK CASCADE jalan (mappings & history ikut terhapus).
- [ ] Test connection screen: spinner + per-host status.
- [ ] Resize terminal: layout tidak crash (handle `tea.WindowSizeMsg`).
- [ ] Manual QA checklist semua ✓.
- [ ] `context7` MCP di-query SEBELUM coding (catat di PR description). Sangat penting untuk bubbletea/bubbles/lipgloss — API berubah cukup sering.

## Blocked by

- Issue 001 (butuh storage, crypto, config).

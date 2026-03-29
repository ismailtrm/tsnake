# tsnake — Implementation Plan

Multiplayer terminal snake (slither.io mekaniği), Go + Charm stack, SSH üzerinden.

---

## Stack

| Katman | Kütüphane |
|--------|-----------|
| SSH server | `github.com/charmbracelet/wish` |
| TUI framework | `github.com/charmbracelet/bubbletea` |
| Styling | `github.com/charmbracelet/lipgloss` |
| SSH primitives | `github.com/charmbracelet/ssh` |

---

## Repo Yapısı

```
tsnake/
├── main.go
├── game/
│   ├── state.go
│   ├── engine.go
│   └── snake.go
├── player/
│   ├── model.go
│   └── renderer.go
├── ui/
│   └── styles.go
├── Dockerfile
├── .github/
│   └── workflows/
│       └── deploy.yml
└── IMPLEMENTATION.md
```

---

## Phase 1 — Local Single Player

> Hedef: SSH olmadan lokal terminalde çalışan snake.

### Task 1.1 — Proje kurulumu
- [ ] `go mod init github.com/<user>/tsnake`
- [ ] Bağımlılıkları ekle:
  ```bash
  go get github.com/charmbracelet/bubbletea
  go get github.com/charmbracelet/lipgloss
  go get github.com/charmbracelet/wish
  go get github.com/charmbracelet/ssh
  ```

### Task 1.2 — `game/snake.go`
- [ ] `Point` struct: `{X, Y int}`
- [ ] `Direction` type: `Up | Down | Left | Right`
- [ ] `Snake` struct:
  ```go
  type Snake struct {
      Body      []Point
      Dir       Direction
      NextDir   Direction
      Color     lipgloss.Color
      Name      string
      Alive     bool
      Score     int
  }
  ```
- [ ] `func (s *Snake) Head() Point`
- [ ] `func (s *Snake) Move()` — head'i NextDir yönünde ilerlet, tail'i kırp
- [ ] `func (s *Snake) Grow()` — tail'i kırpma, body uzasın

### Task 1.3 — `game/state.go`
- [ ] `Game` struct:
  ```go
  type Game struct {
      mu     sync.RWMutex
      W, H   int           // grid boyutu (200x60)
      Snakes map[string]*Snake
      Food   []Point
  }
  ```
- [ ] `func NewGame(w, h int) *Game`
- [ ] `func (g *Game) AddSnake(id, name string) *Snake` — random başlangıç pozisyonu
- [ ] `func (g *Game) RemoveSnake(id string)`
- [ ] `func (g *Game) SetDirection(id string, dir Direction)`
- [ ] `GameSnapshot` struct — mutex olmadan okunabilir kopyası:
  ```go
  type GameSnapshot struct {
      Snakes map[string]SnakeSnap
      Food   []Point
      W, H   int
  }
  ```
- [ ] `func (g *Game) Snapshot() GameSnapshot`

### Task 1.4 — `game/engine.go`
- [ ] `func (g *Game) Tick()`:
  - Her snake için `Move()` çağır
  - Wall collision: `X < 0 || X >= W || Y < 0 || Y >= H` → snake ölür
  - Self collision: head diğer body segmentleriyle çakışıyor mu?
  - Cross collision: head başka snake'in body'siyle çakışıyor mu?
  - Food collision: head food pozisyonundaysa `Grow()`, food sil, yeni food spawn et
  - Ölü snake'leri temizle
- [ ] `func (g *Game) SpawnFood()` — random boş hücreye food ekle (max 20 food)
- [ ] `func StartEngine(g *Game, interval time.Duration) chan GameSnapshot`:
  - Ticker ile `Tick()` çağır
  - Her tick sonrası snapshot'ı channel'a gönder

### Task 1.5 — `ui/styles.go`
- [ ] Renk paleti:
  ```go
  var PlayerColors = []lipgloss.Color{
      "#FF6B6B", "#4ECDC4", "#45B7D1",
      "#96CEB4", "#FFEAA7", "#DDA0DD",
      "#F7DC6F", "#82E0AA",
  }
  var FoodColor   = lipgloss.Color("#FFD700")
  var BGColor     = lipgloss.Color("#0D1117")
  var BorderColor = lipgloss.Color("#30363D")
  var TextColor   = lipgloss.Color("#E6EDF3")
  ```
- [ ] Karakter seti:
  ```go
  const (
      CharSnakeHead  = "◆"
      CharSnakeBody  = "●"
      CharFood       = "✦"
      CharEmpty      = " "
      CharHeadUp     = "▲"
      CharHeadDown   = "▼"
      CharHeadLeft   = "◀"
      CharHeadRight  = "▶"
  )
  ```

### Task 1.6 — `player/renderer.go`
- [ ] `Cell` struct: `{Char string; Color lipgloss.Color}`
- [ ] `Renderer` struct:
  ```go
  type Renderer struct {
      prev   [][]Cell
      width  int
      height int
  }
  ```
- [ ] `func (r *Renderer) Render(snap GameSnapshot, playerID string, w, h int) string`:
  - Oyuncunun snake'inin head'ini merkeze al (viewport)
  - Grid'i Cell slice olarak doldur
  - Diff: önceki frame'den farklı hücreleri bul
  - ANSI escape ile sadece değişen hücreleri yaz
  - Leaderboard ve status bar'ı ekle

### Task 1.7 — `player/model.go`
- [ ] Bubbletea `Model` implement et:
  ```go
  type Model struct {
      game      *game.Game
      playerID  string
      snapCh    <-chan game.GameSnapshot
      lastSnap  game.GameSnapshot
      renderer  *Renderer
      width     int
      height    int
  }
  ```
- [ ] `Init()` — snapshot bekle
- [ ] `Update(msg)`:
  - `tea.KeyMsg`: `w/a/s/d` veya arrow key → `SetDirection()`
  - `tea.WindowSizeMsg`: width/height güncelle
  - `GameSnapshot`: lastSnap güncelle, bir sonraki snapshot'ı bekle
  - `q` / `ctrl+c`: oyundan çık
- [ ] `View()` — `renderer.Render()` çağır

### Task 1.8 — `main.go` (lokal mod)
- [ ] Game oluştur, engine başlat
- [ ] Tek oyuncu ekle
- [ ] `tea.NewProgram(model)` ile çalıştır
- [ ] `go run .` ile test et

**Phase 1 tamamlandığında:** Lokal terminalde ok tuşlarıyla oynanan snake çalışıyor olacak.

---

## Phase 2 — SSH Multiplayer

> Hedef: Birden fazla oyuncu aynı anda farklı terminallerden bağlanabilsin.

### Task 2.1 — `main.go` SSH server'a dönüştür
- [ ] wish server kur:
  ```go
  s, err := wish.NewServer(
      wish.WithAddress(":2222"),
      wish.WithIdleTimeout(10 * time.Minute),
      wish.WithHostKeyPath("./data/host_key"),
      wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
          return true
      }),
      wish.WithPasswordAuth(func(ctx ssh.Context, pass string) bool {
          return true
      }),
      wish.WithMiddleware(
          gameMiddleware(game),
          bm.MiddlewareWithProgramHandler(playerHandler(game), termenv.ANSI256),
      ),
  )
  ```
- [ ] `gameMiddleware`: her bağlantıda `AddSnake()`, disconnect'te `RemoveSnake()`
- [ ] `playerHandler`: o bağlantı için `Model` oluştur, channel'ı bağla

### Task 2.2 — Snapshot broadcast
- [ ] Engine'den gelen tek snapshot'ı tüm aktif oyunculara dağıt:
  ```go
  type Hub struct {
      mu       sync.RWMutex
      channels map[string]chan GameSnapshot
  }
  func (h *Hub) Broadcast(snap GameSnapshot) {
      h.mu.RLock()
      defer h.mu.RUnlock()
      for _, ch := range h.channels {
          select {
          case ch <- snap:
          default: // lag eden client'ı bloke etme
          }
      }
  }
  ```
- [ ] Her oyuncu için `Hub.Register(id)` / `Hub.Unregister(id)`

### Task 2.3 — Oyuncu ismi
- [ ] SSH bağlantısında username al: `ctx.User()`
- [ ] Boşsa veya "git" ise random isim ata (`anon-XXXX`)
- [ ] Bağlanınca `ssh play@ip -p 2222` çalışsın, username snake ismi olsun

### Task 2.4 — Multiplayer test
- [ ] İki terminal aç, ikisinden de bağlan
- [ ] Aynı grid'de iki snake görünsün
- [ ] Birbirine çarpınca ölüm gerçekleşsin

**Phase 2 tamamlandığında:** `ssh play@204.168.183.60 -p 2222` ile oynanan multiplayer snake.

---

## Phase 3 — Görsel Polish

> Hedef: kinetype seviyesi terminal estetik.

### Task 3.1 — Yön-aware snake head
- [ ] Head karakteri yöne göre değişsin:
  ```go
  func headChar(dir Direction) string {
      switch dir {
      case Up:    return "▲"
      case Down:  return "▼"
      case Left:  return "◀"
      case Right: return "▶"
      }
  }
  ```

### Task 3.2 — Gradient snake body
- [ ] Baş parlak, kuyruk soluk:
  - Head: tam renk
  - Body[1-3]: %80 opacity
  - Body[4+]: %50 opacity
  - Lipgloss ile renk interpolasyonu

### Task 3.3 — UI layout
- [ ] Sabit frame:
  ```
  ╭─ TSNAKE ──────────────────── N online ─╮
  │  [viewport 100x32]                      │
  ╰─────────────────────────────────────────╯
  ╭─ TOP 5 ────────╮  ╭─ YOU ─────────────╮
  │ 1. ismail  234 │  │ len:12  rank:#1   │
  │ 2. anon    180 │  │ ██████░░ 234 pts  │
  ╰────────────────╯  ╰───────────────────╯
  ```
- [ ] Lipgloss border, padding, color ile stilize et

### Task 3.4 — Death screen
- [ ] Ölünce bubbletea state değiş: `Playing → Dead`
- [ ] 3 saniyelik ekran:
  ```
  ╭────────────────────╮
  │   YOU DIED         │
  │   Score: 234       │
  │   Rank: #3         │
  │                    │
  │   Respawning in 3s │
  ╰────────────────────╯
  ```
- [ ] 3 saniye sonra otomatik respawn

### Task 3.5 — Food animasyonu
- [ ] Food karakteri dönsün: `✦ ✧ ✦ ✧` (her tick değiştir)

### Task 3.6 — Minimap
- [ ] Sağ alt köşede 20x10'luk minimap
- [ ] Tüm snake'lerin pozisyonunu küçük nokta olarak göster
- [ ] Kendi snake'in farklı renkte

**Phase 3 tamamlandığında:** Görsel olarak güçlü, akıcı bir terminal oyunu.

---

## Phase 4 — Deploy

> Hedef: Sunucuya deploy, otomatik CI/CD.

### Task 4.1 — Dockerfile
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o tsnake .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/tsnake .
EXPOSE 2222
VOLUME ["/app/data"]
CMD ["./tsnake"]
```

### Task 4.2 — GitHub Actions
```yaml
# .github/workflows/deploy.yml
name: Deploy
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build & Push
        run: |
          echo ${{ secrets.GITHUB_TOKEN }} | docker login ghcr.io -u ${{ github.actor }} --password-stdin
          docker build -t ghcr.io/${{ github.repository }}:latest .
          docker push ghcr.io/${{ github.repository }}:latest
      - name: Deploy
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.HETZNER_IP }}
          username: ismail
          key: ${{ secrets.HETZNER_SSH_KEY }}
          script: |
            docker pull ghcr.io/${{ github.repository }}:latest
            docker stop tsnake || true
            docker rm tsnake || true
            docker run -d \
              --name tsnake \
              --restart unless-stopped \
              -p 2222:2222 \
              -v tsnake_data:/app/data \
              ghcr.io/${{ github.repository }}:latest
```

### Task 4.3 — GitHub Secrets
Repo → Settings → Secrets → Actions:
```
HETZNER_IP      = 204.168.183.60
HETZNER_SSH_KEY = (private key içeriği)
```

### Task 4.4 — İlk deploy ve test
- [ ] `git push origin main`
- [ ] Actions'ın geçtiğini gör
- [ ] `ssh play@204.168.183.60 -p 2222` ile bağlan

**Phase 4 tamamlandığında:** `git push` → otomatik deploy.

---

## Teknik Notlar

### Tick Rate
100ms ile başla. Titreme varsa 150ms'ye çek. Smooth hissettiriyorsa 50ms dene.

### Diff Renderer
```go
// ANSI escape: cursor'ı row, col'a taşı
fmt.Fprintf(w, "\033[%d;%dH", row+1, col+1)
// Renk + karakter yaz
fmt.Fprintf(w, "\033[38;2;%d;%d;%dm%s\033[0m", r, g, b, char)
```
Sadece değişen hücreleri yaz, tüm ekranı silme.

### Host Key Persistence
`/app/data/host_key` volume'de tutulur. Her deploy'da aynı key kalır, oyuncular "host changed" uyarısı almaz.

### Graceful Shutdown
```go
c := make(chan os.Signal, 1)
signal.Notify(c, os.Interrupt, syscall.SIGTERM)
<-c
s.Close()
```

---

## Öncelik Sırası

```
Phase 1 → Phase 2 → Phase 4 → Phase 3
```

Phase 3'ü en sona bırak. Önce çalışan bir şey deploy et, sonra güzelleştir.

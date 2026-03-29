## Genel Yapı

```
Player Terminal
    └── ssh play@204.168.183.60 -p 2222
            │
            ▼
    ┌─────────────────┐
    │   wish SSH      │  ← her bağlantı için ayrı goroutine
    │   Server :2222  │
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │  Bubbletea      │  ← per-player UI, input/output
    │  Program        │
    └────────┬────────┘
             │ read/write
             ▼
    ┌─────────────────┐
    │   Game Engine   │  ← tek goroutine, 100ms tick
    │   (shared)      │
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │  Game State     │  ← sync.RWMutex korumalı
    │  - snakes map   │
    │  - food []Point │
    │  - scores       │
    └─────────────────┘
```

---

## Dosya Yapısı

```
tsnake/
├── main.go           → wish server, entry point
├── game/
│   ├── state.go      → Game struct, mutex, snapshot
│   ├── engine.go     → tick loop, collision, food spawn
│   └── snake.go      → Snake struct, movement, growth
├── player/
│   ├── model.go      → bubbletea Model, Update, View
│   └── renderer.go   → diff renderer, karakter seti
├── ui/
│   └── styles.go     → lipgloss renk paleti, karakterler
└── Dockerfile
```

---

## Veri Akışı

**Input (oyuncu → sunucu):**
```
Klavye tuşu
  → bubbletea KeyMsg
    → model.Update()
      → game.SetDirection(playerID, dir)
        → snake.nextDir = dir  (mutex ile)
```

**Tick (sunucu → tüm oyuncular):**
```
100ms timer
  → engine.Tick()
    → her snake'i hareket ettir
    → collision check (wall, self, other)
    → food yenildiyse yeni spawn et
    → GameSnapshot oluştur
    → tüm player channel'larına gönder
      → her bubbletea program rerender
```

**Render (sunucu → oyuncu terminali):**
```
GameSnapshot geldi
  → renderer.Diff(prev, curr)
    → sadece değişen hücreler
      → ANSI escape ile o pozisyona git
        → karakteri yaz
```

---

## Kritik Kararlar

**Tick rate: 100ms**
Slither.io'dan yavaş ama terminal için ideal. 50ms deneriz, titreme olursa 100ms'ye çekeriz.

**Viewport: sabit merkez**
Her oyuncu kendi snake'ini ekranın ortasında görür. Dünya ondan büyük (200x60 grid), kamera snake'in başını takip eder.

**Oyuncu kimliği: SSH public key**
```go
wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
    playerID := ssh.FingerprintSHA256(key)
    return true // herkese izin ver, key'i ID olarak kullan
})
```
Key yoksa (anonymous) random UUID ver.

**Ölüm ve respawn:**
- Ölünce 3 saniyelik death screen (skor göster)
- Sonra otomatik respawn, random pozisyon

**Leaderboard:**
- Sadece in-memory, process restart'ta sıfırlanır
- İleride SQLite eklenebilir

---

## Ekran Düzeni

```
╭─ TSNAKE ──────────────────────────────── 8 online ─╮
│                                                     │
│         [ OYUN ALANI ~100x32 ]                      │
│                                                     │
╰─────────────────────────────────────────────────────╯
╭─ TOP 5 ───────╮  ╭─ YOU ──────────────────────────╮
│ 1. ismail 234 │  │ ismail  len:12  rank:#1         │
│ 2. player 180 │  │ ██████████░░░░░░ 234 pts        │
│ 3. anon   120 │  ╰────────────────────────────────╯
╰───────────────╯
```

---

## MVP Sırası

1. Tek oyunculu çalışan snake (bubbletea, lokal)
2. Wish ile SSH üzerinden aynı şey
3. Shared state + multiplayer
4. Diff renderer + görsel polish
5. Leaderboard + death screen
6. Dockerfile + deploy

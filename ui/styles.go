package ui

import "github.com/charmbracelet/lipgloss"

var PlayerColors = []lipgloss.Color{
	"#FF6B6B",
	"#4ECDC4",
	"#45B7D1",
	"#96CEB4",
	"#FFEAA7",
	"#DDA0DD",
	"#F7DC6F",
	"#82E0AA",
}

var (
	FoodColor          = lipgloss.Color("#FFD700")
	ImmortalFruitColor = lipgloss.Color("#58A6FF")
	MegaFruitColor     = lipgloss.Color("#FF4D6D")
	LeaderColor        = lipgloss.Color("#F2CC60")
	RemnantTintColor   = lipgloss.Color("#6E7681")
	BGColor            = lipgloss.Color("#0D1117")
	BorderColor        = lipgloss.Color("#30363D")
	TextColor          = lipgloss.Color("#E6EDF3")
	MutedColor         = lipgloss.Color("#8B949E")
	GridColor          = lipgloss.Color("#2E353D")
	GridGlow           = lipgloss.Color("#383F49")
	DangerColor        = lipgloss.Color("#FF7B72")
	AccentColor        = lipgloss.Color("#7EE787")
)

const (
	CharSnakeHead     = "◆"
	CharSnakeBody     = "●"
	CharFood          = "✦"
	CharFoodAlt       = "✧"
	CharImmortalFruit = "✺"
	CharMegaFruit     = "✹"
	CharRemnantFood   = "•"
	CharDeathMarker   = "X"
	CharEmpty         = " "
	CharHeadUp        = "▲"
	CharHeadDown      = "▼"
	CharHeadLeft      = "◀"
	CharHeadRight     = "▶"
)

var (
	AppStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(BGColor)

	FrameStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Background(BGColor).
			Padding(0, 1)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Background(BGColor).
			Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	DangerStyle = lipgloss.NewStyle().
			Foreground(DangerColor).
			Bold(true)

	AccentStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)
)

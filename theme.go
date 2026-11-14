package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// palette 是参考 Kero 的配色，按浅色和深色外观区分。
type palette struct {
	dark               bool
	text               ui.Color
	textMuted          ui.Color
	textFaint          ui.Color
	rowHover           ui.Color
	rowPressed         ui.Color
	rowSelected        ui.Color
	rowSelectedPressed ui.Color
	card               ui.Color
	chip               ui.Color
	control            ui.Color
	controlHover       ui.Color
	divider            ui.Color
	accent             ui.Color
	attention          ui.Color
	danger             ui.Color
}

// 全应用共用的动效。颜色用短曲线，位置用可打断的弹簧；系统开启
// “减少动态效果”时，MyGo 会直接落到目标值。
var (
	colorMotion   = ui.Animation{Duration: 140 * time.Millisecond, Ease: ui.EaseOut}
	pressSpring   = ui.Animation{Spring: ui.SpringOptions{Duration: 180 * time.Millisecond}}
	pageSpring    = ui.Animation{Spring: ui.SpringOptions{Duration: 200 * time.Millisecond}}
	overlaySpring = ui.Animation{Spring: ui.SpringOptions{Duration: 240 * time.Millisecond}}
	barSpring     = ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond}}
)

var lightPalette = palette{
	text:               ui.Hex("#1d1d1f"),
	textMuted:          ui.Hex("#6e6e78"),
	textFaint:          ui.Hex("#a3a3ad"),
	rowHover:           ui.RGBA(0, 0, 0, 0.04),
	rowPressed:         ui.RGBA(0, 0, 0, 0.07),
	rowSelected:        ui.Hex("#ffffff"),
	rowSelectedPressed: ui.Hex("#f2f2f5"),
	card:               ui.Hex("#ffffff"),
	chip:               ui.RGBA(0, 0, 0, 0.045),
	control:            ui.Hex("#ffffff"),
	controlHover:       ui.Hex("#f7f7f9"),
	divider:            ui.RGBA(0, 0, 0, 0.06),
	accent:             ui.Hex("#007aff"),
	attention:          ui.Hex("#c27b0f"),
	danger:             ui.Hex("#d93a35"),
}

var darkPalette = palette{
	dark:               true,
	text:               ui.Hex("#ececf0"),
	textMuted:          ui.Hex("#a9a9b4"),
	textFaint:          ui.Hex("#83838f"),
	rowHover:           ui.RGBA(255, 255, 255, 0.05),
	rowPressed:         ui.RGBA(255, 255, 255, 0.09),
	rowSelected:        ui.RGBA(255, 255, 255, 0.10),
	rowSelectedPressed: ui.RGBA(255, 255, 255, 0.15),
	card:               ui.Hex("#232328"),
	chip:               ui.RGBA(255, 255, 255, 0.07),
	control:            ui.Hex("#404047"),
	controlHover:       ui.Hex("#4a4a52"),
	divider:            ui.RGBA(255, 255, 255, 0.07),
	accent:             ui.Hex("#0a84ff"),
	attention:          ui.Hex("#f0a33a"),
	danger:             ui.Hex("#ff6961"),
}

// channelHues 返回频段标签用的紫色和青色。
func channelHues(pal *palette) (purple, teal ui.Color) {
	purple = ui.Hex("#8b7bd8")
	teal = ui.Hex("#5f9e94")
	if pal.dark {
		purple = ui.Hex("#a99bf0")
		teal = ui.Hex("#7fc4b8")
	}
	return purple, teal
}

func paletteFor(c *ui.Context) *palette {
	if c.Theme().Dark {
		return &darkPalette
	}
	return &lightPalette
}

// applyTheme 把 Kero 的强调色和去边框风格带进 MyGo 的控件主题，
// 让焦点环、选择色等与整体一致。
func applyTheme(c *ui.Context, p *palette) {
	t := *c.Theme()
	t.Accent = p.accent
	t.Selection = p.accent.Alpha(0.25)
	t.Border = ui.Transparent
	t.Surface = p.card
	t.SurfaceHover = p.card.Mix(p.text, 0.04)
	t.SurfacePressed = p.card.Mix(p.text, 0.06)
	c.SetTheme(&t)
}

// backdrop 返回 Kero「petal」主题的全窗渐变底色与右上角的柔光。
func backdrop(p *palette) (base, glow ui.LinearGradient) {
	if p.dark {
		return gradient(180, "#240f1d", "#26111a", "#271610"),
			glowOf(ui.RGBA(142, 58, 110, 0.30))
	}
	return gradient(180, "#ebe0f3", "#f3e7f1", "#f7eee4", "#f6efcf"),
		glowOf(ui.RGBA(247, 220, 241, 0.75))
}

func gradient(angle float32, hexes ...string) ui.LinearGradient {
	stops := make([]ui.GradientStop, len(hexes))
	for i, h := range hexes {
		stops[i] = ui.GradientStop{Color: ui.Hex(h), At: float32(i) / float32(len(hexes)-1)}
	}
	return ui.LinearGradient{Angle: angle, Oklab: true, Stops: stops}
}

func glowOf(c ui.Color) ui.LinearGradient {
	return ui.LinearGradient{Angle: 225, Oklab: true, Stops: []ui.GradientStop{
		{Color: c, At: 0},
		{Color: c.Alpha(0), At: 0.5},
	}}
}

// paned 是内容卡片在大底色上的描边：浅色为发丝线加内阴影，
// 深色为受光边缘加柔和投影，与 Kero 的卡片一致。
func paned(e ui.Element, p *palette) ui.Element {
	if p.dark {
		return raised(e, p)
	}
	return e.Border(0.5, ui.RGBA(0, 0, 0, 0.106)).InsetShadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.063))
}

// raised 把表面从底色上抬起一点：选中的侧边栏行、按钮。
func raised(e ui.Element, p *palette) ui.Element {
	if p.dark {
		e = e.Border(1, p.text.Alpha(0.12)).BorderLinearGradient(ui.LinearGradient{
			Angle: 180,
			From:  p.text.Alpha(0.18),
			To:    p.text.Alpha(0.06),
		})
		return e.Shadow(0, 1, 1, 0, ui.RGBA(0, 0, 0, 0.20)).
			Shadow(0, 2, 2, -1, ui.RGBA(0, 0, 0, 0.20)).
			Shadow(0, 4, 6, -3, ui.RGBA(0, 0, 0, 0.24))
	}
	return e.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.07)).
		Shadow(0, 1, 6, 0, ui.RGBA(0, 0, 0, 0.035))
}

// chosen 抬起侧边栏中选中的行：浅色是一圈更暗的环和极浅的阴影，
// 深色则同 raised。
func chosen(e ui.Element, p *palette) ui.Element {
	if p.dark {
		return raised(e, p)
	}
	return e.Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.06)).
		Shadow(0, 0.5, 2, 0, ui.RGBA(0, 0, 0, 0.05))
}

// pressScale 在按下的当帧缩小元素，松开后弹回。AnimateTo 从当前值
// 继续，快速连点也不会跳变。
func pressScale(e ui.Element, pressed bool, amount float32) ui.Element {
	target := float32(1)
	if pressed {
		target = amount
	}
	s := e.AnimateTo("press-scale", target, pressSpring)
	return e.Scale(s, s)
}

// contentSwap 用淡入替换一块内容。旧内容在同一帧让位，新内容从下方
// 轻轻浮入，避免两页文字在过渡中叠在一起；输入不会被锁住。
func contentSwap(c *ui.Context, key string, fn func()) {
	e := ui.Column(c).Key(key).Grow(1).MinHeight(0).MinWidth(0)
	e.Transition(ui.ElementTransition{
		Animation: &pageSpring,
		Enter:     &ui.Motion{Opacity: 0, Y: 6},
	})
	e.Children(fn)
}

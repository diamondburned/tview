package tview

import "github.com/diamondburned/tcell"

// MouseSupport defines wether a component supports accepting mouse events
type MouseSupport interface {
	MouseHandler() func(event *tcell.EventMouse) bool
}

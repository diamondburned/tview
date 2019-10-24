package tview

import "github.com/diamondburned/tcell"

// application exposes the whole application as a singleton. This variable will
// be filled when Newapplication() is called.
var application *Application

// Initialize creates a new application.
func Initialize() *Application {
	if application == nil {
		application = NewApplication()
	}

	return application
}

// Run starts the application and thus the event loop. This function returns
// when Stop() was called.
func Run() error {
	return application.Run()
}

func Stop() {
	application.Stop()
}

func Draw() {
	application.Draw()
}

func ForceDraw() {
	application.ForceDraw()
}

func Suspend(f func() error) bool {
	return application.Suspend(f)
}

func ExecApplication(f func(*Application) bool) {
	application.ExecApplication(f)
}

func QueueEvent(event tcell.Event) {
	application.QueueEvent(event)
}

func SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) {
	application.SetInputCapture(capture)
}

func GetInputCapture() func(event *tcell.EventKey) *tcell.EventKey {
	return application.GetInputCapture()
}

func SetScreen(screen tcell.Screen) {
	application.SetScreen(screen)
}

func SetBeforeDrawFunc(handler func(screen tcell.Screen) bool) {
	application.SetBeforeDrawFunc(handler)
}

func GetBeforeDrawFunc() func(screen tcell.Screen) bool {
	return application.GetBeforeDrawFunc()
}

func SetAfterDrawFunc(handler func(screen tcell.Screen)) {
	application.SetAfterDrawFunc(handler)
}

func GetAfterDrawFunc() func(screen tcell.Screen) {
	return application.GetAfterDrawFunc()
}

func SetRoot(root Primitive, fullscreen bool) {
	application.SetRoot(root, fullscreen)
}

func GetRoot() Primitive {
	application.RLock()
	defer application.RUnlock()

	return application.root
}

func ResizeToFullScreen(p Primitive) {
	application.ResizeToFullScreen(p)
}

func SetFocus(p Primitive) {
	application.SetFocus(p)
}

func GetFocus() Primitive {
	return application.GetFocus()
}

func GetComponentAt(x, y int) *Primitive {
	return application.GetComponentAt(x, y)
}

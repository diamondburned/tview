package tview

import (
	"errors"
	"sync"
	"time"

	"github.com/diamondburned/tcell"
)

// QueueSize is the size of the event/update/redraw channels.
var QueueSize = 100

// RefreshRate controls the max rate for draw calls. The default is 40Hz, as
// most VTE terminals use 40Hz.
var RefreshRate = 40

// application exposes the whole application as a singleton. This variable will be filled
// when Newapplication() is called.
var application *Application

var ErrUnitialized = errors.New("tview is unitialized")

// Application represents the top node of an application.
//
// It is not strictly required to use this class as none of the other classes
// depend on it. However, it provides useful tools to set up an application and
// plays nicely with all widgets.
//
// The following command displays a primitive p on the screen until Ctrl-C is
// pressed:
//
//   if err := tview.NewApplication().SetRoot(p, true).Run(); err != nil {
//       panic(err)
//   }
type Application struct {
	sync.RWMutex

	// Screen is the application's screen. Apart from Run(), this variable should never be
	// set directly. Always use the screenReplacement channel after calling
	// Fini(), to set a new screen (or nil to stop the application).
	Screen tcell.Screen

	MouseSupport bool

	// The primitive which currently has the keyboard focus.
	focus Primitive

	// The root primitive to be seen on the screen.
	root Primitive

	// Whether or not the application resizes the root primitive.
	rootFullscreen bool

	// An optional capture function which receives a key event and returns the
	// event to be forwarded to the default input handler (nil if nothing should
	// be forwarded).
	inputCapture func(event *tcell.EventKey) *tcell.EventKey

	// An optional callback function which is invoked just before the root
	// primitive is drawn.
	beforeDraw func(screen tcell.Screen) bool

	// An optional callback function which is invoked after the root primitive
	// was drawn.
	afterDraw func(screen tcell.Screen)

	// Used to send screen events from separate goroutine to main event loop
	events chan tcell.Event

	// Functions queued from goroutines, used to serialize updates to primitives.
	updates chan func()

	// only draw if true
	draw bool

	// An object that the screen variable will be set to after Fini() was called.
	// Use this channel to set a new screen object for the application
	// (screen.Init() and draw() will be called implicitly). A value of nil will
	// stop the application.
	screenReplacement chan tcell.Screen
}

// Initialize creates a new application.
func Initialize() *Application {
	if application == nil {
		application = &Application{
			events:            make(chan tcell.Event, QueueSize),
			updates:           make(chan func(), QueueSize),
			screenReplacement: make(chan tcell.Screen, 1),
			MouseSupport:      true,
		}
	}

	return application
}

// Run starts the application and thus the event loop. This function returns
// when Stop() was called.
func Run() (err error) {
	if application == nil {
		return ErrUnitialized
	}

	application.Lock()

	// Make application.Screen if there is none yet.
	if application.Screen == nil {
		application.Screen, err = tcell.NewScreen()
		if err != nil {
			application.Unlock()
			return err
		}
		if err = application.Screen.Init(); err != nil {
			application.Unlock()
			return err
		}

		if application.MouseSupport {
			// Enable mouse
			application.Screen.EnableMouse()
		}
	}

	// We catch panics to clean up because they mess up the terminal.
	defer func() {
		if p := recover(); p != nil {
			if application.Screen != nil {
				application.Screen.Fini()
			}
			panic(p)
		}
	}()

	// Draw the screen for the first time.
	application.Unlock()
	application.forceDraw()

	// Separate loop to wait for screen events.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			application.RLock()
			screen := application.Screen
			application.RUnlock()
			if screen == nil {
				// We have no screen. Let's stop.
				QueueEvent(nil)
				break
			}

			// Wait for next event and queue it.
			event := screen.PollEvent()
			if event != nil {
				// Regular event. Queue.
				QueueEvent(event)
				continue
			}

			// A screen was finalized (event is nil). Wait for a new scren.
			screen = <-application.screenReplacement
			if screen == nil {
				// No new screen. We're done.
				QueueEvent(nil)
				return
			}

			// We have a new screen. Keep going.
			application.Lock()
			application.Screen = screen
			application.Unlock()

			// Initialize and draw this screen.
			if err := screen.Init(); err != nil {
				panic(err)
			}

			Draw()
		}
	}()

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(time.Second / time.Duration(RefreshRate))
		for range ticker.C {
			if application.draw {
				application.forceDraw()
				application.draw = false
			}
		}
	}()

	// Start event loop.
EventLoop:
	for {
		select {
		case event := <-application.events:
			if event == nil {
				break EventLoop
			}

			switch event := event.(type) {
			case *tcell.EventKey:
				application.RLock()
				p := application.focus
				inputCapture := application.inputCapture
				application.RUnlock()

				// Intercept keys.
				if inputCapture != nil {
					if event = inputCapture(event); event == nil {
						Draw()
						continue // Don't forward event.
					}
				}

				// Ctrl-C closes the application.
				if event.Key() == tcell.KeyCtrlC {
					Stop()
				}

				// Pass other key events to the currently focused primitive.
				if p != nil {
					if handler := p.InputHandler(); handler != nil {
						handler(event, func(p Primitive) {
							SetFocus(p)
						})

						Draw()
					}
				}

			case *tcell.EventMouse:
				application.RLock()
				atXY := GetComponentAt(event.Position())
				application.RUnlock()

				if atXY == nil {
					continue
				}

				mouseSupport, ok := (*atXY).(MouseSupport)
				if !ok {
					continue
				}

				if handler := mouseSupport.MouseHandler(); handler != nil && handler(event) {
					Draw()
				}

			case *tcell.EventResize:
				application.RLock()
				screen := application.Screen
				application.RUnlock()

				if screen == nil {
					continue
				}

				screen.Clear()
				application.forceDraw()
			}

		// If we have updates, now is the time to execute them.
		case updater := <-application.updates:
			updater()
		}
	}

	// Wait for the event loop to finish.
	wg.Wait()
	application.Screen = nil

	return nil
}

// Stop stops the application, causing Run() to return.
func Stop() {
	application.Lock()
	defer application.Unlock()

	if application.Screen == nil {
		return
	}

	application.Screen.Fini()
	application.Screen = nil
	application.screenReplacement <- nil
}

// Draw refreshes the screen (during the next update cycle). It calls the Draw()
// function of the application's root primitive and then syncs the screen
// buffer.
func Draw() {
	application.draw = true
}

// ForceDraw refreshes the screen immediately. Use this function with caution as
// it may lead to race conditions with updates to primitives in other
// goroutines. It is always preferrable to use Draw() instead. Never call this
// function from a goroutine.
//
// It is safe to call this function during queued updates and direct event
// handling.
func ForceDraw() {
	application.forceDraw()
}

// Suspend temporarily suspends the application by exiting terminal UI mode and
// invoking the provided function "f". When "f" returns nil, terminal UI mode is
// entered again and the application resumes. If "f" returns an error, the
// application safely panics.
//
// A return value of true indicates that the application was suspended and "f"
// was called. If false is returned, the application was already suspended,
// terminal UI mode was not exited, and "f" was not called.
func Suspend(f func() error) bool {
	application.RLock()
	screen := application.Screen
	application.RUnlock()

	if screen == nil {
		return false // Screen has not yet been initialized.
	}

	// Enter suspended mode.
	screen.Fini()

	// Wait for "f" to return.
	if err := f(); err != nil {
		panic(err)
	}

	// Make a new screen.
	s, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}

	application.Lock()
	application.Screen = s
	application.Unlock()

	// One key event will get lost, see https://github.com/rivo/tcell/issues/194
	application.screenReplacement <- screen

	// Continue application loop.
	return true
}

// ExecApplication takes in a function and pass in the application. This is intended
// for widgets/primitives to use to trigger a draw by itself.
func ExecApplication(f func(*Application) bool) {
	if f(application) {
		Draw()
	}
}

// QueueUpdate is used to synchronize access to primitives from non-main
// goroutines. The provided function will be executed as part of the event loop
// and thus will not cause race conditions with other such update functions or
// the Draw() function.
//
// Note that Draw() is not implicitly called after the execution of f as that
// may not be desirable. You can call Draw() from f if the screen should be
// refreshed after each update. Alternatively, use QueueUpdateDraw() to follow
// up with an immediate refresh of the screen.
func QueueUpdate(f func()) {
	application.updates <- f
}

// QueueUpdateDraw works like QueueUpdate() except it refreshes the screen
// immediately after executing f.
func QueueUpdateDraw(f func()) {
	QueueUpdate(func() {
		f()
		Draw()
	})
}

// QueueEvent sends an event to the Application event loop.
//
// It is not recommended for event to be nil.
func QueueEvent(event tcell.Event) {
	application.events <- event
}

// SetInputCapture sets a function which captures all key events before they are
// forwarded to the key event handler of the primitive which currently has
// focus. This function can then choose to forward that key event (or a
// different one) by returning it or stop the key event processing by returning
// nil.
//
// Note that this also affects the default event handling of the application
// itself: Such a handler can intercept the Ctrl-C event which closes the
// applicatoon.
func SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) {
	application.inputCapture = capture
}

// GetInputCapture returns the function installed with SetInputCapture() or nil
// if no such function has been installed.
func GetInputCapture() func(event *tcell.EventKey) *tcell.EventKey {
	return application.inputCapture
}

// SetScreen allows you to provide your own tcell.Screen object. For most
// applications, this is not needed and you should be familiar with
// tcell.Screen when using this function.
//
// This function is typically called before the first call to Run(). Init() need
// not be called on the screen.
func SetScreen(screen tcell.Screen) {
	if screen == nil {
		return // Invalid input. Do nothing.
	}

	application.Lock()
	if application.Screen == nil {
		// Run() has not been called yet.
		application.Screen = screen
		application.Unlock()
		return
	}

	// Run() is already in progress. Exchange screen.
	oldScreen := application.Screen
	application.Unlock()
	oldScreen.Fini()
	application.screenReplacement <- screen
}

// forceDraw actually does what Draw() promises to do.
func (a *Application) forceDraw() *Application {
	a.Lock()
	defer a.Unlock()

	screen := a.Screen
	root := a.root
	fullscreen := a.rootFullscreen
	before := a.beforeDraw
	after := a.afterDraw

	// Maybe we're not ready yet or not anymore.
	if screen == nil || root == nil {
		return a
	}

	// Resize if requested.
	if fullscreen && root != nil {
		width, height := screen.Size()
		root.SetRect(0, 0, width, height)
	}

	// Call before handler if there is one.
	if before != nil {
		if before(screen) {
			screen.Show()
			return a
		}
	}

	// If no background is drawn, clear the old buffer so the old runes aren't
	// there
	if Styles.PrimitiveBackgroundColor == -1 {
		screen.Clear()
	}

	// Draw all primitives.
	root.Draw(screen)

	// Call after handler if there is one.
	if after != nil {
		after(screen)
	}

	// Sync screen.
	screen.Show()

	return a
}

// SetBeforeDrawFunc installs a callback function which is invoked just before
// the root primitive is drawn during screen updates. If the function returns
// true, drawing will not continue, i.e. the root primitive will not be drawn
// (and an after-draw-handler will not be called).
//
// Note that the screen is not cleared by the application. To clear the screen,
// you may call screen.Clear().
//
// Provide nil to uninstall the callback function.
func SetBeforeDrawFunc(handler func(screen tcell.Screen) bool) {
	application.beforeDraw = handler
}

// GetBeforeDrawFunc returns the callback function installed with
// SetBeforeDrawFunc() or nil if none has been installed.
func GetBeforeDrawFunc() func(screen tcell.Screen) bool {
	return application.beforeDraw
}

// SetAfterDrawFunc installs a callback function which is invoked after the root
// primitive was drawn during screen updates.
//
// Provide nil to uninstall the callback function.
func SetAfterDrawFunc(handler func(screen tcell.Screen)) {
	application.afterDraw = handler
}

// GetAfterDrawFunc returns the callback function installed with
// SetAfterDrawFunc() or nil if none has been installed.
func GetAfterDrawFunc() func(screen tcell.Screen) {
	return application.afterDraw
}

// SetRoot sets the root primitive for this application. If "fullscreen" is set
// to true, the root primitive's position will be changed to fill the screen.
//
// This function must be called at least once or nothing will be displayed when
// the application starts.
//
// It also calls SetFocus() on the primitive.
func SetRoot(root Primitive, fullscreen bool) {
	application.Lock()
	application.root = root
	application.rootFullscreen = fullscreen
	if application.Screen != nil {
		application.Screen.Clear()
	}

	application.Unlock()

	SetFocus(root)
}

// GetRoot returns the current root of the application.
func GetRoot() Primitive {
	application.RLock()
	root := application.root
	application.RUnlock()

	return root
}

// ResizeToFullScreen resizes the given primitive such that it fills the entire
// screen.
func ResizeToFullScreen(p Primitive) *Application {
	application.RLock()
	width, height := application.Screen.Size()
	application.RUnlock()

	p.SetRect(0, 0, width, height)

	return application
}

// SetFocus sets the focus on a new primitive. All key events will be redirected
// to that primitive. Callers must ensure that the primitive will handle key
// events.
//
// Blur() will be called on the previously focused primitive. Focus() will be
// called on the new primitive.
func SetFocus(p Primitive) {
	application.Lock()
	if application.focus != nil {
		application.focus.Blur()
	}

	application.focus = p

	if application.Screen != nil {
		application.Screen.HideCursor()
	}
	application.Unlock()

	if p != nil {
		p.Focus(func(p Primitive) {
			SetFocus(p)
		})
	}
}

// GetFocus returns the primitive which has the current focus. If none has it,
// nil is returned.
func GetFocus() Primitive {
	application.RLock()
	defer application.RUnlock()
	return application.focus
}

// GetComponentAt returns the highest level component at the given coordinates
// or zero if no component can be found.
func GetComponentAt(x, y int) *Primitive {
	return getComponentAtRecursively(application.root, x, y)
}

func getComponentAtRecursively(primitive Primitive, x, y int) *Primitive {
	flex, isFlex := primitive.(*Flex)
	if isFlex {
		for _, child := range flex.items {
			found := getComponentAtRecursively(child.Item, x, y)
			if found != nil {
				return found
			}
		}
	}

	grid, isGrid := primitive.(*Grid)
	if isGrid {
		for _, child := range grid.items {
			found := getComponentAtRecursively(child.Item, x, y)
			if found != nil {
				return found
			}
		}
	}

	pages, isPages := primitive.(*Pages)
	if isPages {
		for _, page := range pages.pages {
			if page.Visible {
				found := getComponentAtRecursively(page.Item, x, y)
				if found != nil {
					return found
				}
				break
			}
		}
	}

	componentX, componentY, width, height := primitive.GetRect()
	// Subtracting -1 from height and width, since we got a pixel with coordinate already.
	if componentX <= x && componentY <= y && (componentX+width-1) >= x && (componentY+height-1) >= y {
		return &primitive
	}

	return nil
}

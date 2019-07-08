# Rich Interactive Widgets for Terminal UIs

This Go package provides commonly needed components for terminal based user interfaces.

## Pre-warning

DO NOT USE THIS! JUST DON'T!

## This fork

This fork is made to experiment with reactive UI programming. The objective is to expose enough things to make widgets independent enough. This will greatly reduce clutter in the main code.

## Example usage

```go
package main

func main() {
	// Initialize the TUI. This does NOT draw.
	tview.Initialize()

	// Start the main blocking event loop.
	if err := tview.Run(); err != nil {
		panic(err)
	}
}
```

```go
package primitive

func (p *Primitive) SomeCallback() {
	// Callback things

<<<<<<< HEAD
	tview.QueueApplication(func(app *Application) bool {
		return true // Draw when true
	})
}
```


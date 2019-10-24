package tview

import (
	"math"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/diamondburned/tcell"
	runewidth "github.com/mattn/go-runewidth"
)

// InputField is a one-line box (three lines if there is a title) where the
// user can enter text. Use SetAcceptanceFunc() to accept or reject input,
// SetChangedFunc() to listen for changes, and SetMaskCharacter() to hide input
// from onlookers (e.g. for password input).
//
// The following keys can be used for navigation and editing:
//
//   - Left arrow: Move left by one character.
//   - Right arrow: Move right by one character.
//   - Home, Ctrl-A, Alt-a: Move to the beginning of the line.
//   - End, Ctrl-E, Alt-e: Move to the end of the line.
//   - Alt-left, Alt-b: Move left by one word.
//   - Alt-right, Alt-f: Move right by one word.
//   - Backspace: Delete the character before the cursor.
//   - Delete: Delete the character after the cursor.
//   - Ctrl-K: Delete from the cursor to the end of the line.
//   - Ctrl-W: Delete the last word before the cursor.
//   - Ctrl-U: Delete the entire line.
//
// See https://github.com/rivo/tview/wiki/InputField for an example.
type InputField struct {
	*Box

	// The text that was entered.
	text string

	// The text to be displayed before the input area.
	label string

	// The text to be displayed in the input area when "text" is empty.
	placeholder string

	// The label color.
	labelColor tcell.Color

	// The background color of the input area.
	fieldBackgroundColor tcell.Color

	fieldInvalidBackgroundColor tcell.Color

	// The text color of the input area.
	fieldTextColor tcell.Color

	// The text color of the placeholder.
	placeholderTextColor tcell.Color

	// The screen width of the label area. A value of 0 means use the width of
	// the label text.
	labelWidth int

	// The screen width of the input area. A value of 0 means extend as much as
	// possible.
	fieldWidth int

	// A character to mask entered text (useful for password fields). A value of 0
	// disables masking.
	maskCharacter rune

	// The cursor position as a byte index into the text string.
	CursorPos int

	// The number of bytes of the text string skipped ahead while drawing.
	offset int

	// An optional autocomplete function which receives the current text of the
	// input field and returns a slice of strings to be displayed in a drop-down
	// selection.
	autocomplete func(text string) [][2]string

	autocompleteEntries [][2]string

	// The List object which shows the selectable autocomplete entries. If not
	// nil, the list's main texts represent the current autocomplete entries.
	autocompleteList      *List
	autocompleteListMutex sync.Mutex

	// An optional function which may reject the last character that was entered.
	accept  func(text string, ch rune) bool
	invalid bool

	// An optional function which is called when the input has changed.
	changed func(text string)

	// An optional function which is called when the user indicated that they
	// are done entering text. The key which was pressed is provided (tab,
	// shift-tab, enter, or escape).
	done func(tcell.Key)

	// A callback function set by the Form class and called when the user leaves
	// this form item.
	finished func(tcell.Key)
}

// NewInputField returns a new input field.
func NewInputField() *InputField {
	return &InputField{
		Box:                         NewBox(),
		labelColor:                  Styles.SecondaryTextColor,
		fieldInvalidBackgroundColor: tcell.ColorRed,
		fieldBackgroundColor:        Styles.ContrastBackgroundColor,
		fieldTextColor:              Styles.PrimaryTextColor,
		placeholderTextColor:        Styles.ContrastSecondaryTextColor,
	}
}

// SetText sets the current text of the input field.
func (i *InputField) SetText(text string) *InputField {
	i.text = text
	i.CursorPos = len(text)
	if i.changed != nil {
		i.changed(text)
	}
	return i
}

// GetText returns the current text of the input field.
func (i *InputField) GetText() string {
	return i.text
}

// GetPlaceholder returns the current placeholder of the input field.
func (i *InputField) GetPlaceholder() string {
	return i.placeholder
}

// SetLabel sets the text to be displayed before the input area.
func (i *InputField) SetLabel(label string) *InputField {
	i.label = label
	return i
}

// GetLabel returns the text to be displayed before the input area.
func (i *InputField) GetLabel() string {
	return i.label
}

// SetLabelWidth sets the screen width of the label. A value of 0 will cause the
// primitive to use the width of the label string.
func (i *InputField) SetLabelWidth(width int) *InputField {
	i.labelWidth = width
	return i
}

// SetPlaceholder sets the text to be displayed when the input text is empty.
func (i *InputField) SetPlaceholder(text string) *InputField {
	i.placeholder = text
	return i
}

// SetLabelColor sets the color of the label.
func (i *InputField) SetLabelColor(color tcell.Color) *InputField {
	i.labelColor = color
	return i
}

// SetFieldBackgroundColor sets the background color of the input area.
func (i *InputField) SetFieldBackgroundColor(color tcell.Color) *InputField {
	i.fieldBackgroundColor = color
	return i
}

func (i *InputField) SetFieldInvalidBackgroundColor(color tcell.Color) *InputField {
	i.fieldInvalidBackgroundColor = color
	return i
}

// SetFieldTextColor sets the text color of the input area.
func (i *InputField) SetFieldTextColor(color tcell.Color) *InputField {
	i.fieldTextColor = color
	return i
}

// SetPlaceholderTextColor sets the text color of placeholder text.
func (i *InputField) SetPlaceholderTextColor(color tcell.Color) *InputField {
	i.placeholderTextColor = color
	return i
}

// SetFormAttributes sets attributes shared by all form items.
func (i *InputField) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) FormItem {
	i.labelWidth = labelWidth
	i.labelColor = labelColor
	i.backgroundColor = bgColor
	i.fieldTextColor = fieldTextColor
	i.fieldBackgroundColor = fieldBgColor
	return i
}

// SetFieldWidth sets the screen width of the input area. A value of 0 means
// extend as much as possible.
func (i *InputField) SetFieldWidth(width int) *InputField {
	i.fieldWidth = width
	return i
}

// GetFieldWidth returns this primitive's field width.
func (i *InputField) GetFieldWidth() int {
	return i.fieldWidth
}

// SetMaskCharacter sets a character that masks user input on a screen. A value
// of 0 disables masking.
func (i *InputField) SetMaskCharacter(mask rune) *InputField {
	i.maskCharacter = mask
	return i
}

// SetAutocompleteFunc sets an autocomplete callback function which may return
// strings to be selected from a drop-down based on the current text of the
// input field. The drop-down appears only if len(entries) > 0. The callback is
// invoked in this function and whenever the current text changes or when
// Autocomplete() is called. Entries are cleared when the user selects an entry
// or presses Escape.
//
// Each entry is an array of 2 strings. The first string is what will be placed
// into the box. An optional second string could be provided to display.
func (i *InputField) SetAutocompleteFunc(callback func(currentText string) (entries [][2]string)) *InputField {
	i.autocomplete = callback
	return i
}

// Autocomplete invokes the autocomplete callback (if there is one). If the
// length of the returned autocomplete entries slice is greater than 0, the
// input field will present the user with a corresponding drop-down list the
// next time the input field is drawn.
//
// It is safe to call this function from any goroutine. Note that the input
// field is not redrawn automatically unless called from the main goroutine
// (e.g. in response to events).
func (i *InputField) Autocomplete() *List {
	i.autocompleteListMutex.Lock()
	defer i.autocompleteListMutex.Unlock()
	if i.autocomplete == nil {
		return nil
	}

	// Do we have any autocomplete entries?
	i.autocompleteEntries = i.autocomplete(i.text)
	if len(i.autocompleteEntries) == 0 {
		// No entries, no list.
		i.autocompleteList = nil
		return nil
	}

	// Make a list if we have none.
	if i.autocompleteList == nil {
		i.autocompleteList = NewList()
		i.autocompleteList.ShowSecondaryText(false).
			SetMainTextColor(Styles.PrimitiveBackgroundColor).
			SetSelectedTextColor(Styles.PrimitiveBackgroundColor).
			SetSelectedBackgroundColor(Styles.PrimaryTextColor).
			SetHighlightFullLine(true).
			SetBackgroundColor(Styles.MoreContrastBackgroundColor)
	}

	// Fill it with the entries.
	currentEntry := -1
	i.autocompleteList.Clear()
	for index, entry := range i.autocompleteEntries {
		if entry[1] != "" {
			i.autocompleteList.AddItem(entry[0], "", 0, nil)
		} else {
			i.autocompleteList.AddItem(entry[1], "", 0, nil)
		}

		if currentEntry < 0 && entry[0] == i.text {
			currentEntry = index
		}
	}

	// Set the selection if we have one.
	if currentEntry >= 0 {
		i.autocompleteList.SetCurrentItem(currentEntry)
	}

	return i.autocompleteList
}

// SetAcceptanceFunc sets a handler which may reject the last character that was
// entered (by returning false).
//
// This package defines a number of variables prefixed with InputField which may
// be used for common input (e.g. numbers, maximum text length).
func (i *InputField) SetAcceptanceFunc(handler func(textToCheck string, lastChar rune) bool) *InputField {
	i.accept = handler
	return i
}

// SetChangedFunc sets a handler which is called whenever the text of the input
// field has changed. It receives the current text (after the change).
func (i *InputField) SetChangedFunc(handler func(text string)) *InputField {
	i.changed = handler
	return i
}

// SetDoneFunc sets a handler which is called when the user is done entering
// text. The callback function is provided with the key that was pressed, which
// is one of the following:
//
//   - KeyEnter: Done entering text.
//   - KeyEscape: Abort text input.
//   - KeyTab: Move to the next field.
//   - KeyBacktab: Move to the previous field.
func (i *InputField) SetDoneFunc(handler func(key tcell.Key)) *InputField {
	i.done = handler
	return i
}

// SetFinishedFunc sets a callback invoked when the user leaves this form item.
func (i *InputField) SetFinishedFunc(handler func(key tcell.Key)) FormItem {
	i.finished = handler
	return i
}

// Draw draws this primitive onto the screen.
func (i *InputField) Draw(screen tcell.Screen) {
	i.Box.Draw(screen)

	// Prepare
	x, y, width, height := i.GetInnerRect()
	rightLimit := x + width
	if height < 1 || rightLimit <= x {
		return
	}

	// Draw label.
	if i.labelWidth > 0 {
		labelWidth := i.labelWidth
		if labelWidth > rightLimit-x {
			labelWidth = rightLimit - x
		}
		Print(screen, i.label, x, y, labelWidth, AlignLeft, i.labelColor)
		x += labelWidth
	} else {
		_, drawnWidth := Print(screen, i.label, x, y, rightLimit-x, AlignLeft, i.labelColor)
		x += drawnWidth
	}

	// Draw input area.
	fieldWidth := i.fieldWidth
	if fieldWidth == 0 {
		fieldWidth = math.MaxInt32
	}
	if rightLimit-x < fieldWidth {
		fieldWidth = rightLimit - x
	}

	fieldStyle := tcell.StyleDefault
	if i.invalid { // red background if invalid
		fieldStyle = fieldStyle.Background(i.fieldInvalidBackgroundColor)
	} else {
		fieldStyle = fieldStyle.Background(i.fieldBackgroundColor)
	}

	for index := 0; index < fieldWidth; index++ {
		screen.SetContent(x+index, y, ' ', nil, fieldStyle)
	}

	// Text.
	var cursorScreenPos int
	text := i.text
	if text == "" && i.placeholder != "" {
		// Draw placeholder text.
		Print(screen, Escape(i.placeholder), x, y, fieldWidth, AlignLeft, i.placeholderTextColor)
		i.offset = 0
	} else {
		// Draw entered text.
		if i.maskCharacter > 0 {
			text = strings.Repeat(string(i.maskCharacter), utf8.RuneCountInString(i.text))
		}
		stringWidth := runewidth.StringWidth(text)
		if fieldWidth >= stringWidth {
			// We have enough space for the full text.
			Print(screen, Escape(text), x, y, fieldWidth, AlignLeft, i.fieldTextColor)
			i.offset = 0
			iterateString(text, func(main rune, comb []rune, textPos, textWidth, screenPos, screenWidth int) bool {
				if textPos >= i.CursorPos {
					return true
				}
				cursorScreenPos += screenWidth
				return false
			})
		} else {
			// The text doesn't fit. Where is the cursor?
			if i.CursorPos < 0 {
				i.CursorPos = 0
			} else if i.CursorPos > len(text) {
				i.CursorPos = len(text)
			}
			// Shift the text so the cursor is inside the field.
			var shiftLeft int
			if i.offset > i.CursorPos {
				i.offset = i.CursorPos
			} else if subWidth := runewidth.StringWidth(text[i.offset:i.CursorPos]); subWidth > fieldWidth-1 {
				shiftLeft = subWidth - fieldWidth + 1
			}
			currentOffset := i.offset
			iterateString(text, func(main rune, comb []rune, textPos, textWidth, screenPos, screenWidth int) bool {
				if textPos >= currentOffset {
					if shiftLeft > 0 {
						i.offset = textPos + textWidth
						shiftLeft -= screenWidth
					} else {
						if textPos+textWidth > i.CursorPos {
							return true
						}
						cursorScreenPos += screenWidth
					}
				}
				return false
			})
			Print(screen, Escape(text[i.offset:]), x, y, fieldWidth, AlignLeft, i.fieldTextColor)
		}
	}

	// Draw autocomplete list.
	i.autocompleteListMutex.Lock()
	defer i.autocompleteListMutex.Unlock()
	if i.autocompleteList != nil {
		// How much space do we need?
		lheight := i.autocompleteList.GetItemCount()
		lwidth := 0
		for index := 0; index < lheight; index++ {
			entry, _ := i.autocompleteList.GetItemText(index)
			width := TaggedStringWidth(entry)
			if width > lwidth {
				lwidth = width
			}
		}

		// We prefer to drop down but if there is no space, maybe drop up?
		lx := x
		ly := y + 1
		_, sheight := screen.Size()
		if ly+lheight >= sheight && ly-2 > lheight-ly {
			ly = y - lheight
			if ly < 0 {
				ly = 0
			}
		}
		if ly+lheight >= sheight {
			lheight = sheight - ly
		}
		i.autocompleteList.SetRect(lx, ly, lwidth, lheight)
		i.autocompleteList.Draw(screen)
	}

	// Set cursor.
	if i.focus.HasFocus() {
		screen.ShowCursor(x+cursorScreenPos, y)
	}
}

// InputHandler returns the handler for this primitive.
func (i *InputField) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return i.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p Primitive)) {
		// Trigger changed events.
		currentText := i.text
		defer func() {
			if i.text != currentText {
				i.Autocomplete()
				if i.changed != nil {
					i.changed(i.text)
				}
			}
		}()

		// Movement functions.
		home := func() { i.CursorPos = 0 }
		end := func() { i.CursorPos = len(i.text) }
		moveLeft := func() {
			iterateStringReverse(i.text[:i.CursorPos], func(main rune, comb []rune, textPos, textWidth, screenPos, screenWidth int) bool {
				i.CursorPos -= textWidth
				return true
			})
		}
		moveRight := func() {
			iterateString(i.text[i.CursorPos:], func(main rune, comb []rune, textPos, textWidth, screenPos, screenWidth int) bool {
				i.CursorPos += textWidth
				return true
			})
		}
		moveWordLeft := func() {
			i.CursorPos = len(regexp.MustCompile(`\S+\s*$`).ReplaceAllString(i.text[:i.CursorPos], ""))
		}
		moveWordRight := func() {
			i.CursorPos = len(i.text) - len(regexp.MustCompile(`^\s*\S+\s*`).ReplaceAllString(i.text[i.CursorPos:], ""))
		}

		// Add character function. Returns whether or not the rune character is
		// accepted.
		add := func(r rune) bool {
			newText := i.text[:i.CursorPos] + string(r) + i.text[i.CursorPos:]
			if i.accept != nil && !i.accept(newText, r) {
				i.invalid = true
			} else {
				i.invalid = false
			}

			i.text = newText
			i.CursorPos += len(string(r))
			return true
		}

		// Finish up.
		finish := func(key tcell.Key) {
			if i.invalid {
				return
			}

			if i.done != nil {
				i.done(key)
			}
			if i.finished != nil {
				i.finished(key)
			}
		}

		// Process key event.
		i.autocompleteListMutex.Lock()
		defer i.autocompleteListMutex.Unlock()
		switch key := event.Key(); key {
		case tcell.KeyRune: // Regular character.
			if event.Modifiers()&tcell.ModAlt > 0 {
				// We accept some Alt- key combinations.
				switch event.Rune() {
				case 'a': // Home.
					home()
				case 'e': // End.
					end()
				case 'b': // Move word left.
					moveWordLeft()
				case 'f': // Move word right.
					moveWordRight()
				}
			} else {
				// Other keys are simply accepted as regular characters.
				if !add(event.Rune()) {
					break
				}
			}
		case tcell.KeyCtrlU: // Delete all.
			i.text = ""
			i.CursorPos = 0
		case tcell.KeyCtrlK: // Delete until the end of the line.
			i.text = i.text[:i.CursorPos]
		case tcell.KeyCtrlW: // Delete last word.
			lastWord := regexp.MustCompile(`\S+\s*$`)
			newText := lastWord.ReplaceAllString(i.text[:i.CursorPos], "") + i.text[i.CursorPos:]
			i.CursorPos -= len(i.text) - len(newText)
			i.text = newText
		case tcell.KeyBackspace, tcell.KeyBackspace2: // Delete character before the cursor.
			iterateStringReverse(i.text[:i.CursorPos], func(main rune, comb []rune, textPos, textWidth, screenPos, screenWidth int) bool {
				i.text = i.text[:textPos] + i.text[textPos+textWidth:]
				i.CursorPos -= textWidth
				return true
			})
			if i.offset >= i.CursorPos {
				i.offset = 0
			}
		case tcell.KeyDelete: // Delete character after the cursor.
			iterateString(i.text[i.CursorPos:], func(main rune, comb []rune, textPos, textWidth, screenPos, screenWidth int) bool {
				i.text = i.text[:i.CursorPos] + i.text[i.CursorPos+textWidth:]
				return true
			})
		case tcell.KeyLeft:
			if event.Modifiers()&tcell.ModAlt > 0 {
				moveWordLeft()
			} else {
				moveLeft()
			}
		case tcell.KeyRight:
			if event.Modifiers()&tcell.ModAlt > 0 {
				moveWordRight()
			} else {
				moveRight()
			}
		case tcell.KeyHome, tcell.KeyCtrlA:
			home()
		case tcell.KeyEnd, tcell.KeyCtrlE:
			end()
		case tcell.KeyEnter, tcell.KeyEscape: // We might be done.
			if i.autocompleteList != nil {
				i.autocompleteList = nil
			} else {
				finish(key)
			}
		case tcell.KeyDown, tcell.KeyTab: // Autocomplete selection.
			if i.autocompleteList != nil {
				count := i.autocompleteList.GetItemCount()

				newEntry := i.autocompleteList.GetCurrentItem()
				newEntry++

				if newEntry >= count {
					newEntry = 0
				}

				i.autocompleteList.SetCurrentItem(newEntry)
				i.SetText(i.autocompleteEntries[newEntry][0])
			} else {
				finish(key)
			}
		case tcell.KeyUp, tcell.KeyBacktab: // Autocomplete selection.
			if i.autocompleteList != nil {
				newEntry := i.autocompleteList.GetCurrentItem()
				newEntry--
				if newEntry < 0 {
					newEntry = i.autocompleteList.GetItemCount() - 1
				}

				i.autocompleteList.SetCurrentItem(newEntry)
				i.SetText(i.autocompleteEntries[newEntry][0])
			} else {
				finish(key)
			}
		}
	})
}

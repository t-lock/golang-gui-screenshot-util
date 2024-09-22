package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/kbinani/screenshot"
	"golang.design/x/clipboard"
)

type selectionState struct {
	cursorPos f32.Point
	start     f32.Point
	end       f32.Point
	drawing   mouseButton
	editing   bool
	saving    bool
}

type editorState struct {
	markups        []markup
	currentMarkupI int
}

type markup struct {
	start      f32.Point
	end        f32.Point
	buttonType mouseButton
}

type mouseButton int

const (
	none mouseButton = iota
	left
	right
)

type overwriteChan <-chan struct{}

func main() {
	go func() {
		var selection selectionState
		var editor editorState
		var clipboardChan overwriteChan

		bgImage, err := getScreen()
		if err != nil {
			log.Fatal(err)
		}

		window := new(app.Window)
		window.Option(app.Title("Screenshot"))
		window.Option(app.Decorated(false))
		window.Option(app.Size(1, 1))

		err = loop(window, bgImage, &selection, &editor, &clipboardChan)
		if err != nil {
			log.Fatal(err)
		}
		if !selection.saving {
			os.Exit(0)
		}

		if clipboardChan != nil {
			<-clipboardChan
			os.Exit(0)
		}
	}()

	app.Main()
}

func loop(window *app.Window, bgImage image.Image, selection *selectionState, editor *editorState, clipboardChan *overwriteChan) error {
	var ops op.Ops

	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			// This graphics context is used for managing the rendering state
			gtx := app.NewContext(&ops, e)

			// Draw the bg image
			paint.NewImageOp(bgImage).Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)

			// Make the cursor a cross-hair
			pointer.CursorCrosshair.Add(gtx.Ops)

			// Capture pointer events
			handlePointerEvents(gtx, window, selection, editor)

			// Draw the mask
			drawMask(gtx, selection)

			// Draw the box if we're drawing
			if selection.drawing != none {
				drawBox(&ops, selection)
			}

			// Draw markups if we have them
			drawMarkups(&ops, editor)

			// Save the screenshot if we're saving
			if selection.saving {
				image := cropScreenshot(bgImage, selection)

				// Put image on clipboard
				var err error
				*clipboardChan, err = putImageOnClipboard(image)
				if err != nil {
					log.Fatalf("Failed to put image on clipboard: %v", err)
				}
				window.Perform(system.ActionClose)
			}

			// Make the window fullscreen
			window.Perform(system.ActionCenter)
			window.Perform(system.ActionFullscreen)

			// Pass the drawing operations to the GPU
			e.Frame(gtx.Ops)
		}
	}
}

func getScreen() (image.Image, error) {
	bounds := screenshot.GetDisplayBounds(0)
	return screenshot.CaptureRect(bounds)
}

func cropScreenshot(img image.Image, selection *selectionState) image.Image {
	type SubImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	cropSize := image.Rect(int(selection.start.X), int(selection.start.Y), int(selection.end.X), int(selection.end.Y))
	newImg := img.(SubImager).SubImage(cropSize)

	now := time.Now().Format("2006-01-02_15-04-05")
	file, err := os.Create(now + ".png")
	if err != nil {
		panic(err)
	}

	defer file.Close()
	if err := png.Encode(file, newImg); err != nil {
		panic(err)
	}

	return newImg
}

func putImageOnClipboard(img image.Image) (overwriteChan, error) {
	// Convert image to PNG
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	// Initialize the clipboard
	err := clipboard.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize clipboard: %w", err)
	}

	// Write image to clipboard
	changed := clipboard.Write(clipboard.FmtImage, buf.Bytes())

	return changed, nil
}

func handlePointerEvents(gtx layout.Context, w *app.Window, selection *selectionState, editor *editorState) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: w,
			Kinds:  pointer.Move | pointer.Press | pointer.Drag | pointer.Release,
		})
		if !ok {
			break
		}
		if ev, ok := ev.(pointer.Event); ok {
			switch ev.Kind {
			case pointer.Move:
				selection.cursorPos = ev.Position
			case pointer.Press:
				if ev.Buttons&(pointer.ButtonPrimary|pointer.ButtonSecondary) != 0 {
					if selection.editing {
						markup := markup{
							start:      ev.Position,
							end:        ev.Position,
							buttonType: none,
						}
						switch ev.Buttons {
						case pointer.ButtonPrimary:
							markup.buttonType = left
						case pointer.ButtonSecondary:
							markup.buttonType = right
						}
						editor.markups = append(editor.markups, markup)
						editor.currentMarkupI = len(editor.markups) - 1
					} else {
						selection.start = ev.Position
						selection.end = ev.Position
					}
				}
			case pointer.Drag:
				if ev.Buttons&(pointer.ButtonPrimary|pointer.ButtonSecondary) != 0 {

					if selection.editing {
						editor.markups[editor.currentMarkupI].end = ev.Position
					} else {
						selection.cursorPos = ev.Position
						switch ev.Buttons {
						case pointer.ButtonPrimary:
							selection.drawing = left
						case pointer.ButtonSecondary:
							selection.drawing = right
						}
						selection.end = ev.Position
					}
				}
			case pointer.Release:
				if selection.drawing == right {
					selection.saving = true
					selection.drawing = none
				}

				if selection.drawing == left {
					selection.editing = true
				}
			}
		}
	}

	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, w)
	area.Pop()
}

func drawMask(gtx layout.Context, selection *selectionState) {
	ops := gtx.Ops
	width := float32(gtx.Constraints.Max.X)
	height := float32(gtx.Constraints.Max.Y)
	shade := color.NRGBA{R: 0, G: 0, B: 0, A: 200}

	var path clip.Path
	path.Begin(ops)

	path.LineTo(f32.Pt(width, 0))
	path.LineTo(f32.Pt(width, height))
	path.LineTo(f32.Pt(0, height))
	path.LineTo(f32.Pt(0, 0))

	if selection.drawing == none {
		path.MoveTo(selection.cursorPos)
		path.Move(f32.Pt(50, 0))
		path.ArcTo(selection.cursorPos, selection.cursorPos, -2*math.Pi)
	} else {
		path.MoveTo(selection.start)
		path.LineTo(f32.Pt(selection.start.X, selection.end.Y))
		path.LineTo(selection.end)
		path.LineTo(f32.Pt(selection.end.X, selection.start.Y))
		path.LineTo(selection.start)
	}

	defer clip.Outline{Path: path.End()}.Op().Push(ops).Pop()
	paint.ColorOp{Color: shade}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

func drawBox(ops *op.Ops, selection *selectionState) {
	min := f32.Pt(min(selection.start.X, selection.end.X), min(selection.start.Y, selection.end.Y))
	max := f32.Pt(max(selection.start.X, selection.end.X), max(selection.start.Y, selection.end.Y))

	// b4d455 := color.NRGBA{R: 180, G: 212, B: 85, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	path := clip.Path{}
	path.Begin(ops)
	path.MoveTo(min)
	path.LineTo(f32.Pt(max.X, min.Y))
	path.LineTo(max)
	path.LineTo(f32.Pt(min.X, max.Y))
	path.Close()

	paint.FillShape(ops, white, clip.Stroke{
		Path:  path.End(),
		Width: 1,
	}.Op())
}

func drawMarkups(ops *op.Ops, editor *editorState) {
	for _, markup := range editor.markups {

		min := image.Pt(int(min(markup.start.X, markup.end.X)), int(min(markup.start.Y, markup.end.Y)))
		max := image.Pt(int(max(markup.start.X, markup.end.X)), int(max(markup.start.Y, markup.end.Y)))

		red := color.NRGBA{R: 255, G: 0, B: 0, A: 255}

		if markup.buttonType == left {
			rect := clip.Rect{Min: min, Max: max}
			paint.FillShape(ops, red,
				clip.Stroke{
					Path:  rect.Path(),
					Width: 2,
				}.Op(),
			)
		}

		if markup.buttonType == right {
			// Draw the arrow line
			path := clip.Path{}
			path.Begin(ops)
			path.MoveTo(markup.start)
			path.LineTo(markup.end)
			arrowLine := path.End()

			paint.FillShape(ops, red,
				clip.Stroke{
					Path:  arrowLine,
					Width: 2,
				}.Op())

			// Calculate the angle of the line
			dx := markup.end.X - markup.start.X
			dy := markup.end.Y - markup.start.Y
			angle := math.Atan2(float64(dy), float64(dx))

			// Calculate the arrow head points
			leftPoint := f32.Pt(
				markup.end.X-30*float32(math.Cos(angle))+10*float32(math.Cos(angle+math.Pi/2)),
				markup.end.Y-30*float32(math.Sin(angle))+10*float32(math.Sin(angle+math.Pi/2)),
			)
			rightPoint := f32.Pt(
				markup.end.X-30*float32(math.Cos(angle))+10*float32(math.Cos(angle-math.Pi/2)),
				markup.end.Y-30*float32(math.Sin(angle))+10*float32(math.Sin(angle-math.Pi/2)),
			)

			// Draw the arrow head
			path = clip.Path{}
			path.Begin(ops)
			path.MoveTo(markup.end)
			path.LineTo(leftPoint)
			path.LineTo(rightPoint)
			path.Close()
			paint.FillShape(ops, red, clip.Outline{Path: path.End()}.Op())
		}
	}
}

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
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

type boxState struct {
	start   f32.Point
	end     f32.Point
	drawing mouseButton
	saving  bool
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
		var box boxState
		var clipboardChan overwriteChan

		bgImage, err := getScreen()
		if err != nil {
			log.Fatal(err)
		}

		window := new(app.Window)
		window.Option(app.Title("Screenshot"))
		window.Option(app.Decorated(false))
		window.Option(app.Size(1, 1))

		err = loop(window, &box, bgImage, &clipboardChan)
		if err != nil {
			log.Fatal(err)
		}
		if !box.saving {
			os.Exit(0)
		}

		if clipboardChan != nil {
			<-clipboardChan
			os.Exit(0)
		}
	}()

	app.Main()
}

func loop(window *app.Window, box *boxState, bgImage image.Image, clipboardChan *overwriteChan) error {
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
			handlePointerEvents(gtx, window, box)

			// Draw the box if we're drawing
			if box.drawing != none {
				drawBox(&ops, box.start, box.end)
			}

			// Save the screenshot if we're saving
			if box.saving {
				image := cropScreenshot(bgImage, box.start, box.end)

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

func cropScreenshot(img image.Image, start f32.Point, end f32.Point) image.Image {
	type SubImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	cropSize := image.Rect(int(start.X), int(start.Y), int(end.X), int(end.Y))
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

func handlePointerEvents(gtx layout.Context, w *app.Window, box *boxState) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: w,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release,
		})
		if !ok {
			break
		}
		if ev, ok := ev.(pointer.Event); ok {
			switch ev.Kind {
			case pointer.Press:
				if ev.Buttons&(pointer.ButtonPrimary|pointer.ButtonSecondary) != 0 {
					box.start = ev.Position
					box.end = ev.Position

					switch ev.Buttons {
					case pointer.ButtonPrimary:
						box.drawing = left
					case pointer.ButtonSecondary:
						box.drawing = right
					}
				}
			case pointer.Drag:
				if box.drawing != none {
					box.end = ev.Position
				}
			case pointer.Release:
				if box.drawing == right {
					box.saving = true
				}
				box.drawing = none
			}
		}
	}

	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, w)
	area.Pop()
}

func drawBox(ops *op.Ops, start, end f32.Point) {
	min := f32.Pt(min(start.X, end.X), min(start.Y, end.Y))
	max := f32.Pt(max(start.X, end.X), max(start.Y, end.Y))

	b4d455 := color.NRGBA{R: 180, G: 212, B: 85, A: 255}

	path := clip.Path{}
	path.Begin(ops)
	path.MoveTo(min)
	path.LineTo(f32.Pt(max.X, min.Y))
	path.LineTo(max)
	path.LineTo(f32.Pt(min.X, max.Y))
	path.Close()

	paint.FillShape(ops, b4d455, clip.Stroke{
		Path:  path.End(),
		Width: 1,
	}.Op())
}

package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"

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
)

type boxState struct {
	start   f32.Point
	end     f32.Point
	drawing bool
	saving  bool
}

var tag = new(bool)

func main() {
	go func() {
		window := new(app.Window)
		window.Option(app.Title("Screenshot"))
		window.Perform(system.ActionFullscreen)
		err := loop(window)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(window *app.Window) error {
	var ops op.Ops
	var box boxState

	bgImage, err := getScreen()
	if err != nil {
		return err
	}

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
			handlePointerEvents(gtx, window, &box)

			// Draw the box if we're drawing
			if box.drawing {
				drawBox(&ops, box.start, box.end)
			}

			// Save the screenshot if we're saving
			if box.saving {
				cropScreenshot(bgImage, box.start, box.end)
				os.Exit(0)
			}

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

	file, err := os.Create("cropped.png")
	if err != nil {
		panic(err)
	}

	defer file.Close()
	if err := png.Encode(file, newImg); err != nil {
		panic(err)
	}

	return newImg
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
				if ev.Buttons == pointer.ButtonPrimary {
					box.start = ev.Position
					box.end = ev.Position
					box.drawing = true
					box.saving = false
				}
			case pointer.Drag:
				if box.drawing {
					box.end = ev.Position
					box.saving = false
				}
			case pointer.Release:
				box.drawing = false
				box.saving = true
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

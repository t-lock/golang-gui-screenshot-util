# Golang "selective" screenshot util

Exploring Go and GUI development by creating a screenshotting utility.

[![Video Preview](thumb.png)](https://github.com/user-attachments/assets/c311806c-8a5d-4d1f-bb2e-9d1ec25bd93a)

Recommended usage:

- build executable `go build -o screenshot main.go`
- place your executable somewhere on PATH (eg: `mv screenshot /usr/bin/local`)
- configure a keyboard shortcut for your `screenshot` executable via your system settings


This tool is streamlined for capturing annotated or un-annotated screenshots of a selection of the current window, without any visual clutter. There are no buttons/ui and no clicks/inputs from the user are required beyond the absolute necessities for achieving the current task. Annotations currently supported are limited to red boxes and arrows, but planned features include text, and the ability to change the annotation color and font-size.

This is achieved primarily through usage of right vs left click while dragging, the result of which differs depending on what has already been done. The enitre UX is outlined in the following flow chart:

```
           Start program with keyboard shortcut
                            |
              ________________________________
             |                                |
    Drag selection with        Drag selection with
    left mouse button          right mouse button
             |                                |
      Annotation mode                         |
    _________|__________________              |
   |                            |             |
Left click and         Right click and        |
drag to draw boxes     drag to draw arrows    |
   |____________________________|             |
             |                                |
      Press enter to save                     |
             |________________________________|
                            |
                            |
              Screenshot copied to clipboard 
                    and saved to disk
```

Your screenshot is available on the clipboard, and a file is output to your Home directory on Linux (and probably Mac as well). On Windows I have no idea. If you happen to stumble across this repo and install it on Windows, let me know how it goes!

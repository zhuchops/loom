//go:build wails && darwin

package editor

// Wails' file dialogs use UTType, which lives in its own framework; link it
// here so a plain go build works without CGO_LDFLAGS.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#import <Cocoa/Cocoa.h>

static void loomRaise(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp activateIgnoringOtherApps:YES];
	});
}

// The window's material follows the theme: dark glass behind a dark theme,
// light glass behind a light one.
static void loomAppearance(int dark) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSApp.appearance = [NSAppearance appearanceNamed:dark ? NSAppearanceNameDarkAqua : NSAppearanceNameAqua];
	});
}
*/
import "C"

// raiseWindow brings Loom in front of other apps.
func raiseWindow() { C.loomRaise() }

// setAppearance makes the window's chrome and glass dark or light.
func setAppearance(dark bool) {
	d := C.int(0)
	if dark {
		d = 1
	}
	C.loomAppearance(d)
}

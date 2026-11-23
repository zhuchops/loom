//go:build desktop && darwin

package editor

// Wails' file dialogs use UTType, which lives in its own framework; link it
// here so a plain go build works without CGO_LDFLAGS.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#import <Cocoa/Cocoa.h>

// loomInstallWindow gives the window what the system offers windows: it
// reopens where it was left, and it can go full screen.
static void loomInstallWindow(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *window = NSApp.mainWindow ?: NSApp.windows.firstObject;
		if (window == nil) {
			return;
		}
		window.collectionBehavior |= NSWindowCollectionBehaviorFullScreenPrimary;
		if (![window setFrameUsingName:@"LoomMain"]) {
			[window center];
		}
		[window setFrameAutosaveName:@"LoomMain"];
	});
}

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

// installWindow lets the window remember where it was.
func installWindow() { C.loomInstallWindow() }

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

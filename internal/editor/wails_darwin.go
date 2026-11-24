//go:build desktop && darwin

package editor

// Wails' file dialogs use UTType, which lives in its own framework; link it
// here so a plain go build works without CGO_LDFLAGS.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework UniformTypeIdentifiers
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

// macOS attaches its writing helpers (Writing Tools, the Siri writing
// suggestions, inline predictions) to the text field that has the keyboard,
// wherever it is. The page's only field is the hidden one that takes keys, so
// the web view is made without them: Wails builds its WKWebView with
// initWithFrame:configuration:, which is wrapped here to adjust the
// configuration first. Key-value coding leaves older systems alone.
static IMP loomOriginalInit;

static id loomInitWebView(id self, SEL cmd, NSRect frame, WKWebViewConfiguration *config) {
	if (config != nil) {
		if ([config respondsToSelector:NSSelectorFromString(@"setWritingToolsBehavior:")]) {
			[config setValue:@(-1) forKey:@"writingToolsBehavior"]; // WKWritingToolsBehaviorNone
		}
		if ([config respondsToSelector:NSSelectorFromString(@"setAllowsInlinePredictions:")]) {
			[config setValue:@NO forKey:@"allowsInlinePredictions"];
		}
	}
	return ((id (*)(id, SEL, NSRect, WKWebViewConfiguration *))loomOriginalInit)(self, cmd, frame, config);
}

static void loomQuietWebView(void) {
	if (loomOriginalInit != NULL) {
		return;
	}
	Method m = class_getInstanceMethod([WKWebView class], @selector(initWithFrame:configuration:));
	if (m != NULL) {
		loomOriginalInit = method_setImplementation(m, (IMP)loomInitWebView);
	}
}

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

// loomQuietTyping turns off what macOS offers in text fields: inline
// predictions, completion, corrections and substitutions. The page's only
// field is the hidden one that takes the keyboard, so they only get in the way.
static void loomQuietTyping(void) {
	loomQuietWebView();
	NSUserDefaults *d = [NSUserDefaults standardUserDefaults];
	for (NSString *key in @[ @"NSAutomaticInlinePredictionEnabled", @"NSAutomaticTextCompletionEnabled",
			@"NSAutomaticSpellingCorrectionEnabled", @"NSAutomaticQuoteSubstitutionEnabled",
			@"NSAutomaticDashSubstitutionEnabled", @"NSAutomaticTextReplacementEnabled",
			@"NSAutomaticCapitalizationEnabled", @"NSAutomaticPeriodSubstitutionEnabled",
			@"WebAutomaticInlinePredictionEnabled", @"WebAutomaticSpellingCorrectionEnabled",
			@"WebAutomaticTextReplacementEnabled", @"WebContinuousSpellCheckingEnabled",
			@"WebGrammarCheckingEnabled", @"WebAutomaticQuoteSubstitutionEnabled", @"WebAutomaticDashSubstitutionEnabled" ]) {
		[d setBool:NO forKey:key];
	}
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

// quietTyping turns off the system's typing suggestions for this app.
func quietTyping() { C.loomQuietTyping() }

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

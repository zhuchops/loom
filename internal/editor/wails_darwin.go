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

// The web view's setting doesn't reach the small "write with Siri" panel
// AppKit puts beside the text field that has the keyboard: its controller
// asks one class method whether that panel is on at all, so it answers no.
static BOOL loomNo(id self, SEL cmd) { return NO; }

static void loomQuietCampo(void) {
	Class c = objc_getClass("NSCampoLightweightUIController");
	if (c == Nil) {
		return; // a system without the panel
	}
	Method m = class_getClassMethod(c, NSSelectorFromString(@"_isLightweightUIEnabledForWritingTools"));
	if (m != NULL) {
		method_setImplementation(m, (IMP)loomNo);
	}
}

static void loomQuietWebView(void) {
	loomQuietCampo();
	if (loomOriginalInit != NULL) {
		return;
	}
	Method m = class_getInstanceMethod([WKWebView class], @selector(initWithFrame:configuration:));
	if (m != NULL) {
		loomOriginalInit = method_setImplementation(m, (IMP)loomInitWebView);
	}
}

// loomParentWindow finds the window of the Loom that started this one.
static BOOL loomParentWindow(NSRect *out) {
	pid_t parent = getppid();
	CFArrayRef list = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
	if (list == NULL) {
		return NO;
	}
	BOOL found = NO;
	for (NSDictionary *w in (__bridge NSArray *)list) {
		if ([w[(id)kCGWindowOwnerPID] intValue] != parent || [w[(id)kCGWindowLayer] intValue] != 0) {
			continue;
		}
		CGRect r;
		if (CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)w[(id)kCGWindowBounds], &r) && r.size.width > 200) {
			*out = NSRectFromCGRect(r);
			found = YES;
			break;
		}
	}
	CFRelease(list);
	return found;
}

// loomInstallWindow gives the window what the system offers windows: it
// reopens where it was left, and it can go full screen. Another window
// (cascade) starts down and to the right of that place and is not
// remembered, and comes to the front, as it was started from this app.
static void loomInstallWindow(int cascade) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *window = NSApp.mainWindow ?: NSApp.windows.firstObject;
		if (window == nil) {
			return;
		}
		window.collectionBehavior |= NSWindowCollectionBehaviorFullScreenPrimary;
		if (cascade) {
			NSRect from;
			if (loomParentWindow(&from)) {
				// Screen coordinates there count down from the top of the
				// main screen; the window's count up from its bottom.
				CGFloat top = NSScreen.screens.firstObject.frame.size.height;
				[window setFrame:NSMakeRect(from.origin.x, 0, from.size.width, from.size.height) display:NO];
				[window setFrameTopLeftPoint:NSMakePoint(from.origin.x + 28, top - from.origin.y - 28)];
			} else if ([window setFrameUsingName:@"LoomMain"]) {
				NSRect f = window.frame;
				[window setFrameTopLeftPoint:NSMakePoint(f.origin.x + 28, f.origin.y + f.size.height - 28)];
			} else {
				[window center];
			}
			[NSApp activateIgnoringOtherApps:YES];
			[window makeKeyAndOrderFront:nil];
			return;
		}
		if (![window setFrameUsingName:@"LoomMain"]) {
			[window center];
		}
		[window setFrameAutosaveName:@"LoomMain"];
	});
}

// The Dock menu: New Window, and the recent projects, each in a window of
// its own. Wails' application delegate has no Dock menu, so one is added
// to its class. The picks and the list come from Go (wails_dock_darwin.go).
extern char *loomDockProjects(void);
extern void loomDockOpen(char *path);

@interface LoomDockTarget : NSObject
@end

@implementation LoomDockTarget
- (void)open:(NSMenuItem *)item {
	NSString *path = item.representedObject;
	loomDockOpen((char *)path.UTF8String);
}
@end

static LoomDockTarget *loomDockTarget;

static NSMenuItem *loomDockItem(NSMenu *menu, NSString *title, NSString *path) {
	NSMenuItem *item = [menu addItemWithTitle:title action:@selector(open:) keyEquivalent:@""];
	item.target = loomDockTarget;
	item.representedObject = path;
	return item;
}

static NSMenu *loomDockMenu(id self, SEL cmd, NSApplication *app) {
	NSMenu *menu = [[NSMenu alloc] init];
	menu.autoenablesItems = NO;
	loomDockItem(menu, @"New Window", @"");
	char *list = loomDockProjects();
	NSString *text = list != NULL ? [NSString stringWithUTF8String:list] : @"";
	free(list);
	BOOL first = YES;
	for (NSString *path in [text componentsSeparatedByString:@"\n"]) {
		if (path.length == 0) {
			continue;
		}
		if (first) {
			[menu addItem:[NSMenuItem separatorItem]];
			NSMenuItem *head = [menu addItemWithTitle:@"Open in New Window" action:nil keyEquivalent:@""];
			head.enabled = NO;
			first = NO;
		}
		NSMenuItem *item = loomDockItem(menu, path.lastPathComponent, path);
		item.toolTip = path;
	}
	return menu;
}

static void loomInstallDock(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		id delegate = NSApp.delegate;
		if (delegate == nil || loomDockTarget != nil) {
			return;
		}
		loomDockTarget = [LoomDockTarget new];
		class_addMethod([delegate class], @selector(applicationDockMenu:), (IMP)loomDockMenu, "@@:@");
	});
}

// loomQuietTyping turns off what macOS offers in text fields: inline
// predictions, completion, corrections, substitutions and the accent picker. The page's only
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
	// Holding a letter shows the accent picker instead of repeating the key,
	// so held j or k would not move the cursor. Repeat, as editors do.
	[d setBool:NO forKey:@"ApplePressAndHoldEnabled"];
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
func installWindow(cascade bool) {
	n := C.int(0)
	if cascade {
		n = 1
	}
	C.loomInstallWindow(n)
}

func installDock() { C.loomInstallDock() }

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

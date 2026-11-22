//go:build desktop && darwin && !ios

// What Gio leaves to the application on macOS: the menu bar, the traffic
// lights over our own title bar, file panels and a chance to save before quit.

#import <AppKit/AppKit.h>
#import <objc/runtime.h>

#include "_cgo_export.h"
#include "desktop_darwin.h"

static __weak NSWindow *loomWindow;
static id loomTarget;

@interface LoomMenuTarget : NSObject
@end

@implementation LoomMenuTarget
- (void)perform:(NSMenuItem *)item {
	loomMenuAction((char *)[(NSString *)item.representedObject UTF8String]);
}
@end

static NSApplicationTerminateReply shouldTerminate(id self, SEL cmd, NSApplication *app) {
	loomMenuAction((char *)"quit");
	return NSTerminateCancel;
}

static NSMenu *submenu(NSMenu *bar, NSString *title) {
	NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
	NSMenu *menu = [[NSMenu alloc] initWithTitle:title];
	[item setSubmenu:menu];
	[bar addItem:item];
	return menu;
}

static void item(NSMenu *menu, NSString *title, NSString *command, NSString *key, NSEventModifierFlags mods) {
	// A shifted letter has to be upper case, or it also answers to the plain shortcut.
	NSString *equivalent = (mods & NSEventModifierFlagShift) ? key.uppercaseString : key;
	NSMenuItem *it = [[NSMenuItem alloc] initWithTitle:title action:@selector(perform:) keyEquivalent:equivalent];
	it.target = loomTarget;
	it.representedObject = command;
	if (key.length > 0) {
		it.keyEquivalentModifierMask = mods;
	}
	[menu addItem:it];
}

static void buildMenus(void) {
	const NSEventModifierFlags cmd = NSEventModifierFlagCommand;
	const NSEventModifierFlags shift = cmd | NSEventModifierFlagShift;
	const NSEventModifierFlags option = cmd | NSEventModifierFlagOption;
	NSMenu *bar = [NSMenu new];

	NSMenu *m = submenu(bar, @"Loom");
	[m addItemWithTitle:@"About Loom" action:@selector(orderFrontStandardAboutPanel:) keyEquivalent:@""];
	[m addItem:[NSMenuItem separatorItem]];
	item(m, @"Settings…", @"settings", @",", cmd);
	item(m, @"Color Theme…", @"theme", @"", 0);
	[m addItem:[NSMenuItem separatorItem]];
	[m addItemWithTitle:@"Hide Loom" action:@selector(hide:) keyEquivalent:@"h"];
	NSMenuItem *others = [m addItemWithTitle:@"Hide Others" action:@selector(hideOtherApplications:) keyEquivalent:@"h"];
	others.keyEquivalentModifierMask = option;
	[m addItemWithTitle:@"Show All" action:@selector(unhideAllApplications:) keyEquivalent:@""];
	[m addItem:[NSMenuItem separatorItem]];
	item(m, @"Quit Loom", @"quit", @"q", cmd);

	m = submenu(bar, @"File");
	item(m, @"New File", @"new-file", @"n", cmd);
	item(m, @"Open File…", @"open-file", @"o", cmd);
	item(m, @"Open Project…", @"open-project", @"o", shift);
	item(m, @"Recent Projects…", @"projects", @"", 0);
	[m addItem:[NSMenuItem separatorItem]];
	item(m, @"Save", @"save", @"s", cmd);
	item(m, @"Save As…", @"save-as", @"s", shift);
	[m addItem:[NSMenuItem separatorItem]];
	item(m, @"Close Tab", @"close-tab", @"w", cmd);
	item(m, @"Close Project", @"close-project", @"", 0);

	m = submenu(bar, @"Edit");
	item(m, @"Undo", @"undo", @"z", cmd);
	item(m, @"Redo", @"redo", @"z", shift);
	[m addItem:[NSMenuItem separatorItem]];
	item(m, @"Cut", @"cut", @"x", cmd);
	item(m, @"Copy", @"copy", @"c", cmd);
	item(m, @"Paste", @"paste", @"v", cmd);
	item(m, @"Select All", @"select-all", @"a", cmd);
	[m addItem:[NSMenuItem separatorItem]];
	item(m, @"Find…", @"search", @"f", cmd);
	item(m, @"Replace…", @"replace", @"f", option);
	item(m, @"Find in Project…", @"project-search", @"f", shift);

	m = submenu(bar, @"View");
	item(m, @"Command Palette…", @"palette", @"p", shift);
	item(m, @"Toggle Sidebar", @"sidebar", @"b", cmd);
	item(m, @"Source Control", @"git", @"g", shift);
	item(m, @"Problems", @"problems", @"m", shift);
	[m addItem:[NSMenuItem separatorItem]];
	item(m, @"Increase Font Size", @"zoom-in", @"=", cmd);
	item(m, @"Decrease Font Size", @"zoom-out", @"-", cmd);
	item(m, @"Reset Font Size", @"zoom-reset", @"0", cmd);
	[m addItem:[NSMenuItem separatorItem]];
	NSMenuItem *full = [m addItemWithTitle:@"Enter Full Screen" action:@selector(toggleFullScreen:) keyEquivalent:@"f"];
	full.keyEquivalentModifierMask = cmd | NSEventModifierFlagControl;

	m = submenu(bar, @"Go");
	item(m, @"Go to File…", @"files", @"p", cmd);
	item(m, @"Go to Line…", @"goto-line", @"l", cmd);
	item(m, @"Welcome", @"welcome", @"", 0);

	m = submenu(bar, @"Window");
	[m addItemWithTitle:@"Minimize" action:@selector(performMiniaturize:) keyEquivalent:@"m"];
	[m addItemWithTitle:@"Zoom" action:@selector(performZoom:) keyEquivalent:@""];
	[NSApp setWindowsMenu:m];

	m = submenu(bar, @"Help");
	item(m, @"Keyboard Shortcuts", @"shortcuts", @"", 0);
	[NSApp setHelpMenu:m];

	[NSApp setMainMenu:bar];
}

void loomInstallWindow(uintptr_t ref) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSView *view = (__bridge NSView *)(void *)ref;
		NSWindow *window = view.window;
		if (window == nil || window == loomWindow) {
			return;
		}
		loomWindow = window;
		loomTarget = [LoomMenuTarget new];
		buildMenus();

		id delegate = [NSApp delegate];
		if (delegate != nil) {
			class_addMethod([delegate class], @selector(applicationShouldTerminate:), (IMP)shouldTerminate, "Q@:@");
		}

		// Gio hides the buttons when it draws no decorations; we only want the bar gone.
		for (NSNumber *b in @[ @(NSWindowCloseButton), @(NSWindowMiniaturizeButton), @(NSWindowZoomButton) ]) {
			[window standardWindowButton:(NSWindowButton)b.integerValue].hidden = NO;
		}
		window.titleVisibility = NSWindowTitleHidden;
		window.titlebarAppearsTransparent = YES;
		window.collectionBehavior |= NSWindowCollectionBehaviorFullScreenPrimary;
		// The first run is centred; later ones reopen where the window was left.
		if (![window setFrameUsingName:@"LoomMain"]) {
			[window center];
		}
		[window setFrameAutosaveName:@"LoomMain"];
	});
}

void loomSetAppearance(int dark, int r, int g, int b) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *window = loomWindow;
		if (window == nil) {
			return;
		}
		window.appearance = [NSAppearance appearanceNamed:dark ? NSAppearanceNameDarkAqua : NSAppearanceNameAqua];
		window.backgroundColor = [NSColor colorWithSRGBRed:r / 255.0 green:g / 255.0 blue:b / 255.0 alpha:1];
	});
}

void loomPickPath(int request, int kind, const char *start) {
	NSString *dir = [NSString stringWithUTF8String:start];
	dispatch_async(dispatch_get_main_queue(), ^{
		NSSavePanel *panel;
		if (kind == 2) {
			panel = [NSSavePanel savePanel];
			panel.canCreateDirectories = YES;
		} else {
			NSOpenPanel *open = [NSOpenPanel openPanel];
			open.canChooseDirectories = kind == 0;
			open.canChooseFiles = kind == 1;
			open.allowsMultipleSelection = NO;
			open.canCreateDirectories = kind == 0;
			open.prompt = kind == 0 ? @"Open Project" : @"Open";
			panel = open;
		}
		if (dir.length > 0) {
			panel.directoryURL = [NSURL fileURLWithPath:dir isDirectory:YES];
		}
		void (^done)(NSModalResponse) = ^(NSModalResponse response) {
			const char *path = response == NSModalResponseOK ? panel.URL.path.UTF8String : "";
			loomPickedPath(request, (char *)(path ? path : ""));
		};
		if (loomWindow != nil) {
			[panel beginSheetModalForWindow:loomWindow completionHandler:done];
		} else {
			[panel beginWithCompletionHandler:done];
		}
	});
}

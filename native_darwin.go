//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

@interface WPWebView : WKWebView
@end
@implementation WPWebView
- (void)zoomIn:(id)s    { self.pageZoom = MIN(self.pageZoom + 0.1, 2.0); }
- (void)zoomOut:(id)s   { self.pageZoom = MAX(self.pageZoom - 0.1, 0.6); }
- (void)zoomReset:(id)s { self.pageZoom = 1.0; }
@end

static NSWindow *gWin;
static WPWebView *gWeb;
static NSStatusItem *gStatus;
static NSMenuItem *gInfo1, *gInfo2;
static NSString *gBase;
static BOOL gAlert;

// A round wasaping dot for the menu bar: green, red when a critical device is down.
static NSImage *dotImage(BOOL alert) {
	NSImage *img = [NSImage imageWithSize:NSMakeSize(16, 16) flipped:NO drawingHandler:^BOOL(NSRect r) {
		NSColor *c = alert ? [NSColor colorWithRed:0.84 green:0.25 blue:0.25 alpha:1]
		                   : [NSColor colorWithRed:0.506 green:0.843 blue:0.259 alpha:1]; // #81d742
		[c setFill];
		[[NSBezierPath bezierPathWithOvalInRect:NSInsetRect(r, 3, 3)] fill];
		return YES;
	}];
	img.template = NO;
	return img;
}

static void runJS(NSString *js) {
	[gWeb evaluateJavaScript:js completionHandler:nil];
}

@interface WPDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate, WKUIDelegate, WKNavigationDelegate>
@end

@implementation WPDelegate
// Closing the window keeps wasaping in the menu bar; ⌘Q or "Esci" quit it.
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)app { return NO; }
- (BOOL)windowShouldClose:(NSWindow *)w {
	[w orderOut:nil];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	return NO;
}
- (BOOL)applicationShouldHandleReopen:(NSApplication *)app hasVisibleWindows:(BOOL)flag {
	[self showWindow:nil];
	return YES;
}
- (void)showWindow:(id)sender {
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	[gWin makeKeyAndOrderFront:nil];
	[NSApp activateIgnoringOtherApps:YES];
}
// Scan: in the window when it is open (you see it happen), else in background.
- (void)scanNow:(id)sender {
	if (gWin.isVisible) { runJS(@"window.wasapingScan && wasapingScan()"); return; }
	NSMutableURLRequest *req = [NSMutableURLRequest requestWithURL:[NSURL URLWithString:[gBase stringByAppendingString:@"api/bg?now=1"]]];
	req.HTTPMethod = @"POST";
	[[[NSURLSession sharedSession] dataTaskWithRequest:req] resume];
}
- (void)findInList:(id)sender { [self showWindow:nil]; runJS(@"window.wasapingFind && wasapingFind()"); }
- (void)openMonitor:(id)sender { [self showWindow:nil]; runJS(@"window.wasapingMonitor && wasapingMonitor()"); }

// target=_blank links (device web panels) open in the user's browser.
- (WKWebView *)webView:(WKWebView *)wv createWebViewWithConfiguration:(WKWebViewConfiguration *)cfg
    forNavigationAction:(WKNavigationAction *)action windowFeatures:(WKWindowFeatures *)features {
	if (action.request.URL) [[NSWorkspace sharedWorkspace] openURL:action.request.URL];
	return nil;
}

// Any navigation away from the local server also goes to the browser.
- (void)webView:(WKWebView *)wv decidePolicyForNavigationAction:(WKNavigationAction *)action
    decisionHandler:(void (^)(WKNavigationActionPolicy))handler {
	NSURL *u = action.request.URL;
	NSString *host = u.host ?: @"";
	if (action.targetFrame.isMainFrame && ![host isEqualToString:@"127.0.0.1"] && ![u.scheme isEqualToString:@"about"]) {
		[[NSWorkspace sharedWorkspace] openURL:u];
		handler(WKNavigationActionPolicyCancel);
		return;
	}
	handler(WKNavigationActionPolicyAllow);
}

- (void)webView:(WKWebView *)wv runJavaScriptAlertPanelWithMessage:(NSString *)msg
    initiatedByFrame:(WKFrameInfo *)frame completionHandler:(void (^)(void))done {
	NSAlert *a = [NSAlert new];
	a.messageText = msg;
	[a runModal];
	done();
}
@end

static WPDelegate *gDelegate;

static NSMenuItem *item(NSString *title, SEL action, NSString *key) {
	NSMenuItem *i = [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key];
	return i;
}
static NSMenuItem *mine(NSString *title, SEL action, NSString *key) {
	NSMenuItem *i = item(title, action, key);
	i.target = gDelegate;
	return i;
}

static void buildMenu(void) {
	NSMenu *bar = [NSMenu new];
	NSString *name = [[NSProcessInfo processInfo] processName];

	NSMenu *app = [NSMenu new];
	[app addItem:item([@"Informazioni su " stringByAppendingString:name], @selector(orderFrontStandardAboutPanel:), @"")];
	[app addItem:[NSMenuItem separatorItem]];
	[app addItem:item([@"Nascondi " stringByAppendingString:name], @selector(hide:), @"h")];
	[app addItem:[NSMenuItem separatorItem]];
	[app addItem:item([@"Esci da " stringByAppendingString:name], @selector(terminate:), @"q")];

	NSMenu *edit = [[NSMenu alloc] initWithTitle:@"Modifica"];
	[edit addItem:item(@"Annulla", @selector(undo:), @"z")];
	[edit addItem:item(@"Ripeti", @selector(redo:), @"Z")];
	[edit addItem:[NSMenuItem separatorItem]];
	[edit addItem:item(@"Taglia", @selector(cut:), @"x")];
	[edit addItem:item(@"Copia", @selector(copy:), @"c")];
	[edit addItem:item(@"Incolla", @selector(paste:), @"v")];
	[edit addItem:item(@"Seleziona tutto", @selector(selectAll:), @"a")];

	NSMenu *scan = [[NSMenu alloc] initWithTitle:@"Scansione"];
	[scan addItem:mine(@"Scansiona", @selector(scanNow:), @"r")];
	[scan addItem:mine(@"Cerca nell'elenco", @selector(findInList:), @"f")];
	NSMenuItem *mon = mine(@"Monitor", @selector(openMonitor:), @"m");
	mon.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagShift;
	[scan addItem:mon];

	NSMenu *view = [[NSMenu alloc] initWithTitle:@"Vista"];
	NSMenuItem *reload = item(@"Ricarica la pagina", @selector(reload:), @"r");
	reload.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagShift;
	[view addItem:reload];
	[view addItem:item(@"Ingrandisci", @selector(zoomIn:), @"+")];
	[view addItem:item(@"Rimpicciolisci", @selector(zoomOut:), @"-")];
	[view addItem:item(@"Dimensioni reali", @selector(zoomReset:), @"0")];

	NSMenu *win = [[NSMenu alloc] initWithTitle:@"Finestra"];
	[win addItem:item(@"Riduci a icona", @selector(performMiniaturize:), @"m")];
	[win addItem:item(@"Chiudi (resta nella barra dei menu)", @selector(performClose:), @"w")];

	for (NSMenu *m in @[app, edit, scan, view, win]) {
		NSMenuItem *top = [NSMenuItem new];
		top.submenu = m;
		[bar addItem:top];
	}
	NSApp.mainMenu = bar;
	NSApp.windowsMenu = win;
}

static void buildStatusItem(void) {
	gStatus = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
	gStatus.button.image = dotImage(NO);
	gStatus.button.toolTip = @"wasaping";
	NSMenu *m = [NSMenu new];
	[m addItem:mine(@"Apri wasaping", @selector(showWindow:), @"")];
	[m addItem:mine(@"Scansiona ora", @selector(scanNow:), @"")];
	[m addItem:mine(@"Monitor", @selector(openMonitor:), @"")];
	[m addItem:[NSMenuItem separatorItem]];
	gInfo1 = [[NSMenuItem alloc] initWithTitle:@"Nessuna scansione ancora" action:nil keyEquivalent:@""];
	gInfo2 = [[NSMenuItem alloc] initWithTitle:@"" action:nil keyEquivalent:@""];
	gInfo1.enabled = gInfo2.enabled = NO;
	gInfo2.hidden = YES;
	[m addItem:gInfo1];
	[m addItem:gInfo2];
	[m addItem:[NSMenuItem separatorItem]];
	[m addItem:item(@"Esci da wasaping", @selector(terminate:), @"q")];
	m.autoenablesItems = NO;
	for (NSMenuItem *i in m.itemArray) if (i.action) i.enabled = YES;
	gStatus.menu = m;
}

// setStatus updates the menu bar after a scan (called from any thread).
static void setStatus(const char *l1, const char *l2, int alert) {
	NSString *a = [NSString stringWithUTF8String:l1], *b = [NSString stringWithUTF8String:l2];
	dispatch_async(dispatch_get_main_queue(), ^{
		gInfo1.title = a;
		gInfo2.title = b;
		gInfo2.hidden = b.length == 0;
		if ((BOOL)alert != gAlert) { gAlert = alert; gStatus.button.image = dotImage(gAlert); }
		gStatus.button.toolTip = b.length ? [NSString stringWithFormat:@"wasaping — %@", b] : @"wasaping";
	});
}

static void runWindow(const char *curl, const char *cbase) {
	@autoreleasepool {
		[NSApplication sharedApplication];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		gDelegate = [WPDelegate new];
		NSApp.delegate = gDelegate;
		gBase = [NSString stringWithUTF8String:cbase];
		buildMenu();
		buildStatusItem();

		gWin = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 1120, 720)
			styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable
			backing:NSBackingStoreBuffered defer:NO];
		gWin.title = @"wasaping";
		gWin.minSize = NSMakeSize(640, 420);
		gWin.releasedWhenClosed = NO;
		gWin.delegate = gDelegate;
		[gWin center];
		[gWin setFrameAutosaveName:@"wasaping"];

		WKWebViewConfiguration *cfg = [WKWebViewConfiguration new];
		gWeb = [[WPWebView alloc] initWithFrame:gWin.contentView.bounds configuration:cfg];
		gWeb.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
		gWeb.UIDelegate = gDelegate;
		gWeb.navigationDelegate = gDelegate;
		[gWeb setValue:@NO forKey:@"drawsBackground"]; // no white flash in dark mode
		gWin.contentView = gWeb;
		[gWeb loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:[NSString stringWithUTF8String:curl]]]];

		[gWin makeKeyAndOrderFront:nil];
		[NSApp activateIgnoringOtherApps:YES];
		[NSApp run];
	}
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

// Cocoa must run on the main thread: keep the main goroutine there.
func init() { runtime.LockOSThread() }

const hasNativeWindow = true

// nativeWindow shows the interface in a WebKit window, with an icon in the
// menu bar, and returns only when the user quits. It must be called from
// the main goroutine.
func nativeWindow(url string) {
	onScanDone = updateMenuBar
	cu, cb := C.CString(url), C.CString(url)
	defer C.free(unsafe.Pointer(cu))
	defer C.free(unsafe.Pointer(cb))
	C.runWindow(cu, cb)
}

// updateMenuBar shows the latest scan in the menu bar icon's menu.
func updateMenuBar(r *ScanResult) {
	l1 := fmt.Sprintf("Ultima scansione: %s · %d dispositivi", r.At.Format("15:04"), r.Count)
	var bits []string
	if r.New > 0 {
		bits = append(bits, fmt.Sprintf("%d nuovi", r.New))
	}
	down := 0
	for _, g := range r.Gone {
		if g.Critical {
			down++
		}
	}
	if down > 0 {
		bits = append(bits, fmt.Sprintf("⚠️ %d critici non rispondono", down))
	}
	alert := 0
	if down > 0 {
		alert = 1
	}
	c1, c2 := C.CString(l1), C.CString(strings.Join(bits, " · "))
	defer C.free(unsafe.Pointer(c1))
	defer C.free(unsafe.Pointer(c2))
	C.setStatus(c1, c2, C.int(alert))
}

//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

@interface WPDelegate : NSObject <NSApplicationDelegate, WKUIDelegate, WKNavigationDelegate>
@end

@implementation WPDelegate
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)app { return YES; }

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

static NSMenuItem *item(NSString *title, SEL action, NSString *key) {
	return [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key];
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

	NSMenu *view = [[NSMenu alloc] initWithTitle:@"Vista"];
	[view addItem:item(@"Ricarica", @selector(reload:), @"r")];
	[view addItem:item(@"Ingrandisci", @selector(zoomIn:), @"+")];
	[view addItem:item(@"Rimpicciolisci", @selector(zoomOut:), @"-")];
	[view addItem:item(@"Dimensioni reali", @selector(zoomReset:), @"0")];

	NSMenu *win = [[NSMenu alloc] initWithTitle:@"Finestra"];
	[win addItem:item(@"Riduci a icona", @selector(performMiniaturize:), @"m")];
	[win addItem:item(@"Chiudi", @selector(performClose:), @"w")];

	for (NSMenu *m in @[app, edit, view, win]) {
		NSMenuItem *top = [NSMenuItem new];
		top.submenu = m;
		[bar addItem:top];
	}
	NSApp.mainMenu = bar;
	NSApp.windowsMenu = win;
}

@interface WPWebView : WKWebView
@end
@implementation WPWebView
- (void)zoomIn:(id)s    { self.pageZoom = MIN(self.pageZoom + 0.1, 2.0); }
- (void)zoomOut:(id)s   { self.pageZoom = MAX(self.pageZoom - 0.1, 0.6); }
- (void)zoomReset:(id)s { self.pageZoom = 1.0; }
@end

static void runWindow(const char *curl) {
	@autoreleasepool {
		[NSApplication sharedApplication];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		WPDelegate *d = [WPDelegate new];
		NSApp.delegate = d;
		buildMenu();

		NSWindow *w = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 1120, 720)
			styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable
			backing:NSBackingStoreBuffered defer:NO];
		w.title = @"wasaping";
		w.minSize = NSMakeSize(640, 420);
		w.releasedWhenClosed = NO;
		[w center];
		[w setFrameAutosaveName:@"wasaping"];

		WKWebViewConfiguration *cfg = [WKWebViewConfiguration new];
		WPWebView *wv = [[WPWebView alloc] initWithFrame:w.contentView.bounds configuration:cfg];
		wv.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
		wv.UIDelegate = d;
		wv.navigationDelegate = d;
		[wv setValue:@NO forKey:@"drawsBackground"]; // no white flash in dark mode
		w.contentView = wv;
		[wv loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:[NSString stringWithUTF8String:curl]]]];

		[w makeKeyAndOrderFront:nil];
		[NSApp activateIgnoringOtherApps:YES];
		[NSApp run];
	}
}
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// Cocoa must run on the main thread: keep the main goroutine there.
func init() { runtime.LockOSThread() }

const hasNativeWindow = true

// nativeWindow shows the interface in a WebKit window and returns when the
// user closes it. It must be called from the main goroutine.
func nativeWindow(url string) {
	cs := C.CString(url)
	defer C.free(unsafe.Pointer(cs))
	C.runWindow(cs)
}

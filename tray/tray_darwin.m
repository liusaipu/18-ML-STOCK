#import <Cocoa/Cocoa.h>

static NSStatusItem *statusItem = nil;
static id menuDelegate = nil;
static NSImage *appIconImage = nil;
static NSArray *quotesArray = nil;
static NSInteger currentQuoteIndex = 0;
static CGFloat scrollOffset = 0;
static NSTimer *scrollTimer = nil;
static BOOL trayScrollEnabled = YES;
static BOOL trayIconVisible = YES;
static NSMenuItem *scrollMenuItem = nil;
static NSMenuItem *iconMenuItem = nil;

static CGFloat const SCROLL_SPEED = 35.0;     // 像素/秒
static CGFloat const SCROLL_FPS = 30.0;       // 帧率
static CGFloat const LOGO_SIZE = 22.0;
static CGFloat const PADDING = 4.0;
static CGFloat const VISIBLE_WIDTH = 70.0;    // 文字滚动区域宽度

// 前向声明（在 @implementation 中会被调用）
static void stopScrollTimer(void);
static void tickScrollFrame(void);

// Go 导出函数
extern void trayShowWindow();
extern void trayQuitApp();
extern void trayMenuStateChanged(int scrollEnabled, int iconVisible);

@interface TrayMenuDelegate : NSObject
- (void)showWindow:(id)sender;
- (void)quitApp:(id)sender;
- (void)toggleScroll:(id)sender;
- (void)toggleIcon:(id)sender;
@end

@implementation TrayMenuDelegate
- (void)showWindow:(id)sender {
    trayShowWindow();
}
- (void)quitApp:(id)sender {
    trayQuitApp();
}
- (void)toggleScroll:(id)sender {
    trayScrollEnabled = !trayScrollEnabled;
    [scrollMenuItem setTitle:trayScrollEnabled ? @"关闭滚动字幕" : @"显示滚动字幕"];
    if (!trayScrollEnabled) {
        stopScrollTimer();
        NSButton *button = statusItem.button;
        if (button) {
            // 关闭滚动字幕后只显示纯粹图标，收窄宽度
            NSImage *staticImage = [NSImage imageWithSize:NSMakeSize(LOGO_SIZE + 8, 22) flipped:NO drawingHandler:^BOOL(NSRect dstRect) {
                if (appIconImage) {
                    NSRect logoRect = NSMakeRect(0, (22 - LOGO_SIZE) / 2.0, LOGO_SIZE, LOGO_SIZE);
                    [appIconImage drawInRect:logoRect];
                }
                return YES;
            }];
            button.image = staticImage;
            button.title = @"";
            statusItem.length = LOGO_SIZE + 8;
        }
    } else {
        if (quotesArray && quotesArray.count > 0) {
            currentQuoteIndex = 0;
            scrollOffset = 0;
            tickScrollFrame();
            NSTimeInterval interval = 1.0 / SCROLL_FPS;
            scrollTimer = [NSTimer timerWithTimeInterval:interval
                                                   repeats:YES
                                                     block:^(NSTimer * _Nonnull timer) {
                                                         tickScrollFrame();
                                                     }];
            [[NSRunLoop currentRunLoop] addTimer:scrollTimer forMode:NSRunLoopCommonModes];
        }
    }
    trayMenuStateChanged(trayScrollEnabled ? 1 : 0, trayIconVisible ? 1 : 0);
}
- (void)toggleIcon:(id)sender {
    trayIconVisible = !trayIconVisible;
    [iconMenuItem setTitle:trayIconVisible ? @"隐藏菜单图标" : @"显示菜单图标"];
    statusItem.visible = trayIconVisible;
    trayMenuStateChanged(trayScrollEnabled ? 1 : 0, trayIconVisible ? 1 : 0);
}
@end

void setupTrayIcon(const char* iconPath, const char* tooltip) {
    NSString *tooltipStr = tooltip ? [NSString stringWithUTF8String:tooltip] : @"StockFinLens 财报透镜";
    NSString *iconPathStr = iconPath ? [NSString stringWithUTF8String:iconPath] : nil;

    dispatch_async(dispatch_get_main_queue(), ^{
        NSStatusBar *bar = [NSStatusBar systemStatusBar];
        if (!bar) {
            NSLog(@"[Tray] NSStatusBar is nil");
            return;
        }

        statusItem = [bar statusItemWithLength:NSVariableStatusItemLength];
        if (!statusItem) {
            NSLog(@"[Tray] NSStatusItem is nil");
            return;
        }

        statusItem.visible = YES;

        NSButton *button = statusItem.button;
        if (!button) {
            NSLog(@"[Tray] button is nil");
            return;
        }

        // 加载图标
        if (iconPathStr) {
            appIconImage = [[NSImage alloc] initWithContentsOfFile:iconPathStr];
        }

        if (!appIconImage) {
            appIconImage = [[NSImage alloc] initWithSize:NSMakeSize(18, 18)];
            [appIconImage lockFocus];
            [[NSColor colorWithDeviceRed:0.2 green:0.5 blue:0.9 alpha:1.0] set];
            NSRectFill(NSMakeRect(0, 0, 18, 18));
            [appIconImage unlockFocus];
        }

        appIconImage.template = NO;

        // 从一开始就用固定宽度的渲染帧，只显示图标，避免启动时图标大小异常
        NSImage *initialImage = [NSImage imageWithSize:NSMakeSize(LOGO_SIZE + 8, 22) flipped:NO drawingHandler:^BOOL(NSRect dstRect) {
            if (appIconImage) {
                NSRect logoRect = NSMakeRect(0, (22 - LOGO_SIZE) / 2.0, LOGO_SIZE, LOGO_SIZE);
                [appIconImage drawInRect:logoRect];
            }
            return YES;
        }];
        button.image = initialImage;
        button.title = @"";
        button.toolTip = tooltipStr;
        statusItem.length = LOGO_SIZE + 8;

        NSLog(@"[Tray] button configured: image=%@ title=%@ frame=%@",
              button.image ? @"YES" : @"NO",
              button.title,
              NSStringFromRect(button.frame));

        // 菜单
        menuDelegate = [[TrayMenuDelegate alloc] init];
        NSMenu *menu = [[NSMenu alloc] init];

        NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"显示主窗口"
                                                          action:@selector(showWindow:)
                                                   keyEquivalent:@""];
        showItem.target = menuDelegate;
        [menu addItem:showItem];

        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *scrollItem = [[NSMenuItem alloc] initWithTitle:@"关闭滚动字幕"
                                                              action:@selector(toggleScroll:)
                                                       keyEquivalent:@""];
        scrollItem.target = menuDelegate;
        [menu addItem:scrollItem];
        scrollMenuItem = scrollItem;

        NSMenuItem *iconItem = [[NSMenuItem alloc] initWithTitle:@"隐藏菜单图标"
                                                            action:@selector(toggleIcon:)
                                                     keyEquivalent:@""];
        iconItem.target = menuDelegate;
        [menu addItem:iconItem];
        iconMenuItem = iconItem;

        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"退出"
                                                         action:@selector(quitApp:)
                                                  keyEquivalent:@"q"];
        quitItem.target = menuDelegate;
        [menu addItem:quitItem];

        statusItem.menu = menu;

        NSLog(@"[Tray] === status item ready ===");
    });
}

// 渲染单帧滚动图片：logo 固定左侧，文字在裁剪区域内从右向左滚动
static NSImage* renderScrollFrame(NSString *text, NSColor *color, CGFloat offset) {
    NSFont *font = [NSFont menuBarFontOfSize:14];
    if (!font) {
        font = [NSFont systemFontOfSize:14];
    }

    NSDictionary *attrs = @{
        NSFontAttributeName: font,
        NSForegroundColorAttributeName: color
    };

    NSSize textSize = [text sizeWithAttributes:attrs];
    CGFloat textWidth = MAX(ceil(textSize.width), 1.0);
    CGFloat textHeight = MAX(ceil(textSize.height), 1.0);

    CGFloat canvasWidth = LOGO_SIZE + PADDING + VISIBLE_WIDTH;

    NSImage *image = [NSImage imageWithSize:NSMakeSize(canvasWidth, 22) flipped:NO drawingHandler:^BOOL(NSRect dstRect) {
        // 绘制 logo（固定左侧，垂直居中）
        if (appIconImage) {
            NSRect logoRect = NSMakeRect(0, (22 - LOGO_SIZE) / 2.0, LOGO_SIZE, LOGO_SIZE);
            [appIconImage drawInRect:logoRect];
        }

        // 设置裁剪区域（只显示文字滚动区域）
        NSRect textArea = NSMakeRect(LOGO_SIZE + PADDING, 0, VISIBLE_WIDTH, 22);
        [NSGraphicsContext saveGraphicsState];
        NSRectClip(textArea);

        // 文字位置：从右向左滚动
        // offset=0 时，文字右侧对齐 textArea 右侧（完全在右侧外部开始）
        // offset 增加，文字向左移动
        CGFloat textX = textArea.origin.x + textArea.size.width - offset;
        NSPoint point = NSMakePoint(textX, (22 - textHeight) / 2.0);
        [text drawAtPoint:point withAttributes:attrs];

        [NSGraphicsContext restoreGraphicsState];

        return YES;
    }];

    return image;
}

// 更新滚动帧（由 NSTimer 定时调用）
static void tickScrollFrame() {
    if (!statusItem || !statusItem.button) {
        return;
    }

    if (!quotesArray || quotesArray.count == 0) {
        return;
    }

    NSButton *button = statusItem.button;

    NSDictionary *quote = quotesArray[currentQuoteIndex];
    NSString *name = quote[@"Name"] ?: @"";
    NSString *code = quote[@"Code"] ?: @"";
    NSNumber *priceNum = quote[@"CurrentPrice"];
    NSNumber *changeNum = quote[@"ChangePercent"];

    double price = priceNum.doubleValue;
    double change = changeNum.doubleValue;

    // 格式化文字：名称 价格 涨跌幅
    NSString *displayName = name.length > 0 ? name : code;
    NSString *priceStr = price > 0 ? [NSString stringWithFormat:@"¥%.2f ", price] : @"¥-- ";
    NSString *changeStr = @"";
    if (change > 0) {
        changeStr = [NSString stringWithFormat:@"+%.2f%%", change];
    } else if (change < 0) {
        changeStr = [NSString stringWithFormat:@"%.2f%%", change];
    } else {
        changeStr = @"0.00%";
    }

    NSString *text = [NSString stringWithFormat:@"%@ %@%@", displayName, priceStr, changeStr];

    // 统一使用白色，确保在深色菜单栏上清晰可读
    NSColor *textColor = [NSColor whiteColor];

    // 计算文字宽度（用于判断滚动结束）
    NSFont *font = [NSFont menuBarFontOfSize:14] ?: [NSFont systemFontOfSize:14];
    NSDictionary *attrs = @{NSFontAttributeName: font, NSForegroundColorAttributeName: textColor};
    NSSize textSize = [text sizeWithAttributes:attrs];
    CGFloat textWidth = MAX(ceil(textSize.width), 1.0);

    // 总滚动距离 = 可见区域宽度 + 文字宽度（从右侧完全外部到左侧完全外部）
    CGFloat totalScrollDistance = VISIBLE_WIDTH + textWidth;

    // 更新偏移量
    scrollOffset += SCROLL_SPEED / SCROLL_FPS;

    if (scrollOffset >= totalScrollDistance) {
        // 完全移出，切换到下一只股票
        currentQuoteIndex = (currentQuoteIndex + 1) % quotesArray.count;
        scrollOffset = 0;
        // 递归调用，立即渲染新股票的第一帧
        tickScrollFrame();
        return;
    }

    // 渲染当前帧
    NSImage *frameImage = renderScrollFrame(text, textColor, scrollOffset);
    button.image = frameImage;
    button.title = @"";

    // 固定 status item 宽度，避免跳动
    statusItem.length = LOGO_SIZE + PADDING + VISIBLE_WIDTH;
}

// 停止滚动动画
static void stopScrollTimer() {
    if (scrollTimer) {
        [scrollTimer invalidate];
        scrollTimer = nil;
    }
}

// 设置滚动显示的股票数据（JSON 字符串）
void setTrayQuotes(const char* json) {
    NSString *jsonStr = json ? [NSString stringWithUTF8String:json] : @"[]";
    NSData *data = [jsonStr dataUsingEncoding:NSUTF8StringEncoding];
    NSError *error = nil;
    NSArray *arr = [NSJSONSerialization JSONObjectWithData:data options:0 error:&error];

    dispatch_async(dispatch_get_main_queue(), ^{
        if (error || !arr) {
            NSLog(@"[Tray] failed to parse quotes JSON: %@", error);
            return;
        }

        BOOL hadQuotes = (quotesArray && quotesArray.count > 0);
        NSInteger oldCount = hadQuotes ? (NSInteger)quotesArray.count : 0;

        quotesArray = arr;

        // 只有在首次启动或自选股数量变化时才重置索引，
        // 否则保持当前滚动进度，确保能完整轮播一遍
        if (!hadQuotes || oldCount != (NSInteger)arr.count) {
            currentQuoteIndex = 0;
            scrollOffset = 0;
        }

        // 停止旧的 timer
        stopScrollTimer();

        if (quotesArray.count > 0 && trayScrollEnabled) {
            // 立即渲染第一帧
            tickScrollFrame();

            // 启动定时器
            NSTimeInterval interval = 1.0 / SCROLL_FPS;
            scrollTimer = [NSTimer timerWithTimeInterval:interval
                                                   repeats:YES
                                                     block:^(NSTimer * _Nonnull timer) {
                                                         tickScrollFrame();
                                                     }];
            [[NSRunLoop currentRunLoop] addTimer:scrollTimer forMode:NSRunLoopCommonModes];
        } else if (quotesArray.count > 0 && !trayScrollEnabled) {
            // 滚动已禁用，保持静态显示，收窄宽度
            NSButton *button = statusItem.button;
            if (button) {
                NSImage *staticImage = [NSImage imageWithSize:NSMakeSize(LOGO_SIZE + 8, 22) flipped:NO drawingHandler:^BOOL(NSRect dstRect) {
                    if (appIconImage) {
                        NSRect logoRect = NSMakeRect(0, (22 - LOGO_SIZE) / 2.0, LOGO_SIZE, LOGO_SIZE);
                        [appIconImage drawInRect:logoRect];
                    }
                    return YES;
                }];
                button.image = staticImage;
                button.title = @"";
                statusItem.length = LOGO_SIZE + 8;
            }
        } else {
            // 空数据，恢复默认显示，收窄宽度
            NSButton *button = statusItem.button;
            if (button) {
                NSImage *staticImage = [NSImage imageWithSize:NSMakeSize(LOGO_SIZE + 8, 22) flipped:NO drawingHandler:^BOOL(NSRect dstRect) {
                    if (appIconImage) {
                        NSRect logoRect = NSMakeRect(0, (22 - LOGO_SIZE) / 2.0, LOGO_SIZE, LOGO_SIZE);
                        [appIconImage drawInRect:logoRect];
                    }
                    return YES;
                }];
                button.image = staticImage;
                button.title = @"";
                statusItem.length = LOGO_SIZE + 8;
            }
        }

        NSLog(@"[Tray] received %lu quotes", (unsigned long)arr.count);
    });
}

// 兼容旧接口：显示静态文字（停止滚动）
void updateTrayTitle(const char* title, double changePercent) {
    NSString *titleStr = title ? [NSString stringWithUTF8String:title] : @"SFL";

    dispatch_async(dispatch_get_main_queue(), ^{
        if (!statusItem || !statusItem.button) {
            return;
        }

        // 停止滚动
        stopScrollTimer();

        NSButton *button = statusItem.button;

        NSColor *textColor = [NSColor whiteColor];

        NSFont *font = [NSFont menuBarFontOfSize:14] ?: [NSFont systemFontOfSize:14];
        NSDictionary *attrs = @{
            NSFontAttributeName: font,
            NSForegroundColorAttributeName: textColor
        };

        NSSize textSize = [titleStr sizeWithAttributes:attrs];
        CGFloat textWidth = MAX(ceil(textSize.width), 1.0);
        CGFloat textHeight = MAX(ceil(textSize.height), 1.0);

        CGFloat totalWidth = LOGO_SIZE + PADDING + textWidth;

        NSImage *image = [NSImage imageWithSize:NSMakeSize(totalWidth, 22) flipped:NO drawingHandler:^BOOL(NSRect dstRect) {
            if (appIconImage) {
                NSRect logoRect = NSMakeRect(0, (22 - LOGO_SIZE) / 2.0, LOGO_SIZE, LOGO_SIZE);
                [appIconImage drawInRect:logoRect];
            }
            NSPoint point = NSMakePoint(LOGO_SIZE + PADDING, (22 - textHeight) / 2.0);
            [titleStr drawAtPoint:point withAttributes:attrs];
            return YES;
        }];

        button.image = image;
        button.title = @"";
        statusItem.length = NSVariableStatusItemLength;

        NSLog(@"[Tray] updated static title: %@ (change=%.2f%%)", titleStr, changePercent);
    });
}

// Go 可调用的 C 接口：控制滚动字幕开关
void setTrayScrollEnabled(int enabled) {
    trayScrollEnabled = enabled;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (scrollMenuItem) {
            [scrollMenuItem setTitle:trayScrollEnabled ? @"关闭滚动字幕" : @"显示滚动字幕"];
        }
        if (!trayScrollEnabled) {
            stopScrollTimer();
            NSButton *button = statusItem.button;
            if (button) {
                // 关闭滚动后只显示纯粹图标，收窄宽度
                NSImage *staticImage = [NSImage imageWithSize:NSMakeSize(LOGO_SIZE + 8, 22) flipped:NO drawingHandler:^BOOL(NSRect dstRect) {
                    if (appIconImage) {
                        NSRect logoRect = NSMakeRect(0, (22 - LOGO_SIZE) / 2.0, LOGO_SIZE, LOGO_SIZE);
                        [appIconImage drawInRect:logoRect];
                    }
                    return YES;
                }];
                button.image = staticImage;
                button.title = @"";
                statusItem.length = LOGO_SIZE + 8;
            }
        } else {
            if (quotesArray && quotesArray.count > 0) {
                currentQuoteIndex = 0;
                scrollOffset = 0;
                tickScrollFrame();
                NSTimeInterval interval = 1.0 / SCROLL_FPS;
                scrollTimer = [NSTimer timerWithTimeInterval:interval
                                                       repeats:YES
                                                         block:^(NSTimer * _Nonnull timer) {
                                                             tickScrollFrame();
                                                         }];
                [[NSRunLoop currentRunLoop] addTimer:scrollTimer forMode:NSRunLoopCommonModes];
            }
        }
    });
}

// Go 可调用的 C 接口：控制菜单图标显示/隐藏
void setTrayIconVisible(int visible) {
    trayIconVisible = visible;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (iconMenuItem) {
            [iconMenuItem setTitle:trayIconVisible ? @"隐藏菜单图标" : @"显示菜单图标"];
        }
        if (statusItem) {
            statusItem.visible = trayIconVisible;
        }
    });
}

int isTrayScrollEnabled() {
    return trayScrollEnabled ? 1 : 0;
}

int isTrayIconVisible() {
    return trayIconVisible ? 1 : 0;
}

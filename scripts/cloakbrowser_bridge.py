#!/usr/bin/env python3
"""Narrow JSON bridge between OpenHome's Go runtime and CloakBrowser."""

import json
import os
import sys
from pathlib import Path

MAX_TEXT = 24 * 1024
MAX_LINKS = 60
MAX_NAVIGATION_LINKS = 30


def main() -> None:
    try:
        from cloakbrowser import launch_persistent_context
    except ImportError as exc:
        raise RuntimeError(
            "cloakbrowser is not installed. Run: .venv/bin/python -m pip install cloakbrowser"
        ) from exc

    request = json.load(sys.stdin)
    profile = Path(os.getenv("OPENHOME_CLOAKBROWSER_PROFILE", ".openhome/cloakbrowser-profile"))
    profile.mkdir(parents=True, exist_ok=True)
    headless = os.getenv("OPENHOME_CLOAKBROWSER_HEADLESS", "true").lower() not in {"0", "false", "no"}
    context = launch_persistent_context(str(profile), headless=headless, humanize=True)
    try:
        page = context.new_page()
        page.goto(request["url"], wait_until="domcontentloaded", timeout=30_000)
        wait_for_render(page)
        for action in request.get("actions", []):
            perform(page, action)
        wait_for_render(page)
        text = page.locator("body").inner_text(timeout=10_000)
        links = page.locator("a[href]").evaluate_all(
            """els => {
                const seen = new Set();
                return els.map(a => ({
                    text: (a.innerText || a.textContent || '').replace(/\\s+/g, ' ').trim(),
                    url: a.href,
                    priority: a.closest('main, article, [role="main"]') ? 0 : 1
                })).filter(link => {
                    if (!link.text || !link.url || seen.has(link.url)) return false;
                    seen.add(link.url);
                    return true;
                }).sort((a, b) => a.priority - b.priority)
                  .slice(0, 60)
                  .map(({text, url}) => ({text: text.slice(0, 240), url}));
            }"""
        )
        navigation_links = page.locator("nav a[href], header a[href]").evaluate_all(
            """els => {
                const seen = new Set();
                return els.map(a => ({
                    text: (a.innerText || a.textContent || '').replace(/\\s+/g, ' ').trim(),
                    url: a.href,
                    priority: new URL(a.href).pathname.split('/').filter(Boolean).length
                })).filter(link => {
                    if (!link.text || !link.url || seen.has(link.url)) return false;
                    seen.add(link.url);
                    return true;
                }).sort((a, b) => a.priority - b.priority)
                  .slice(0, 30)
                  .map(({text, url}) => ({text: text.slice(0, 120), url}));
            }"""
        )
        result = {
            "url": page.url,
            "title": page.title(),
            "text": text[:MAX_TEXT],
            "truncated": len(text) > MAX_TEXT,
            "links": links[:MAX_LINKS],
            "navigationLinks": navigation_links[:MAX_NAVIGATION_LINKS],
        }
        screenshot_path = request.get("screenshotOutputPath")
        if screenshot_path:
            screenshot = Path(screenshot_path)
            screenshot.parent.mkdir(parents=True, exist_ok=True)
            for selector in request.get("screenshotHideSelectors", []):
                page.locator(selector).evaluate_all(
                    "els => els.forEach(el => el.style.setProperty('display', 'none', 'important'))"
                )
            selector = request.get("screenshotSelector")
            if selector:
                page.locator(selector).first.screenshot(path=str(screenshot), timeout=10_000)
            else:
                page.screenshot(path=str(screenshot), full_page=bool(request.get("fullPage", False)))
            result["screenshotCaptured"] = True
        print(json.dumps(result))
    finally:
        context.close()


def perform(page, action: dict) -> None:
    kind = action["action"]
    selector = action.get("selector")
    value = action.get("value", "")
    locator = page.locator(selector).first if selector else None
    if kind == "click" and locator:
        locator.click(timeout=10_000)
    elif kind == "fill" and locator:
        locator.fill(value, timeout=10_000)
    elif kind == "type" and locator:
        locator.type(value, timeout=10_000)
    elif kind == "press" and locator:
        locator.press(value, timeout=10_000)
    elif kind == "wait":
        page.wait_for_timeout(min(int(action.get("ms", 500)), 10_000))
    elif kind == "goto":
        page.goto(action["url"], wait_until="domcontentloaded", timeout=30_000)
    else:
        raise ValueError(f"invalid browser action: {action}")


def wait_for_render(page) -> None:
    try:
        page.wait_for_function(
            "() => document.body && document.body.innerText.trim().length > 100",
            timeout=8_000,
        )
    except Exception:
        # Some useful pages are legitimately sparse. Return their final DOM
        # instead of failing the entire browser operation.
        pass


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)

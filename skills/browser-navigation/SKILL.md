---
name: browser-navigation
description: Navigate rendered websites with CloakBrowser, including clicking elements and filling fields when explicitly needed.
allowedTools:
  - webSearch
  - fetchURL
  - browserInteract
---

# Browser Navigation

Use `fetchURL` first for static pages. Use `browserInteract` when the site needs
rendered JavaScript, links must be clicked, or fields must be filled.

Keep each browser interaction narrow. Inspect returned visible text and links,
then perform the next justified action. Use CSS selectors that target a specific
element.

When the user requests a specific section, category, account, or resource, you
MUST NOT answer from a broader homepage merely because it contains related
content. Inspect returned `navigationLinks` and open the matching destination
with another read-only `browserInteract` call before answering. Do not complete
until the returned page URL is the requested destination.

When the user requests a screenshot, set `screenshotPath` to a workspace-relative
PNG path such as `screenshots/article.png`. Use `screenshotSelector` to capture
the article element when a stable CSS selector is available. Otherwise use
`fullPage: true` for the complete rendered page. When a cookie notice or overlay
obstructs the screenshot, use `screenshotHideSelectors` to hide it for capture
without clicking acceptance or changing site state.

Never click buttons that submit purchases, accept legal terms, delete data,
publish content, send messages, or confirm irreversible actions unless the user
explicitly requested that exact action. Read-only page opens run automatically
in default permission mode. Clicks, typing, and additional navigation pause for
permission.

## Completion

Report the resulting page state, relevant URL, and screenshot artifact path.
State clearly when a page could not be reached or an interaction was not
performed.
